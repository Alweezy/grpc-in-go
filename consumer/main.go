package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"grpc-in-go/util"
	"grpc-in-go/util/logger"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"
)

// Prometheus metrics
var (
	tasksProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tasks_processed_total",
		Help: "Total number of tasks processed by the consumer service",
	})
	taskProcessingFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "task_processing_failures_total",
		Help: "Total number of task processing failures",
	})
	tasksByType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_by_type_total",
			Help: "Total number of tasks processed by task type",
		},
		[]string{"task_type"},
	)
	taskValuesByType = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "task_values_by_type_total",
			Help: "Total sum of task values processed by task type",
		},
		[]string{"task_type"},
	)
	tasksInProcessing = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tasks_in_processing",
		Help: "Number of tasks currently being processed by the consumer",
	})
)

// Map to store the total sum of task values by type
var (
	taskTypeSums   = make(map[int32]float64)
	taskTypeSumsMu sync.Mutex // Mutex for thread-safe access to the map
)

func init() {
	// Register the Prometheus metrics
	prometheus.MustRegister(tasksProcessed)
	prometheus.MustRegister(taskProcessingFailures)
	prometheus.MustRegister(tasksByType)
	prometheus.MustRegister(taskValuesByType)
	prometheus.MustRegister(tasksInProcessing)
}

type server struct {
	pb.UnimplementedTaskServiceServer
	limiter *rate.Limiter
	queries *persistence.Queries
}

// Config struct to hold configuration values
type Config struct {
	Database    Database          `mapstructure:"database"`
	Consumer    Consumer          `mapstructure:"consumer"`
	Prometheus  Prometheus        `mapstructure:"prometheus"`
	Logger      *logger.LogConfig `mapstructure:"logger"`
	RateLimiter RateLimiter       `mapstructure:"rate_limiter"`
}

type Database struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DbName   string `mapstructure:"dbname"`
	SslMode  string `mapstructure:"sslmode"`
}

type Consumer struct {
	Port          int `mapstructure:"port"`
	ProfilingPort int `mapstructure:"profiling_port"`
	GrpcPort      int `mapstructure:"grpc_port"`
}

type Prometheus struct {
	ScrapeInterval string `mapstructure:"scrape_interval"`
}

type RateLimiter struct {
	TasksPerSecond float64 `mapstructure:"tasks_per_second"`
}

var version string

func main() {

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Printf("Consumer service version: %s\n", version)
		return
	}

	// Load the configuration using the util function
	appConfig := &util.AppConfig{
		FilePath: "configs",
		FileName: "consumer",
		Type:     util.ConfigYAML,
	}

	// Define a Config struct to hold the loaded configuration
	var config Config

	// Load the configuration using the utility function
	if err := util.LoadConfig(appConfig, &config); err != nil {
		logger.LogError("Failed to load configuration", err, &logger.LogContext{
			"service": "consumer",
		})
		return
	}

	// Initialize logger
	logFormatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	err := logger.LoadLogger(config.Logger, logFormatter)
	if err != nil {
		logger.LogError("Failed to initialize logger", err, &logger.LogContext{
			"service": "consumer",
		})
		return
	}

	logger.LogInfo("Consumer service starting", &logger.LogContext{
		"version": version,
	})

	// Start the Prometheus metrics endpoint in a separate goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logger.LogInfo("Prometheus metrics available", &logger.LogContext{
			"port": config.Consumer.Port,
			"url":  fmt.Sprintf("http://localhost:%d/metrics", config.Consumer.Port),
		})
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Consumer.Port), nil); err != nil {
			logger.LogError("Failed to start Prometheus metrics server", err, &logger.LogContext{
				"port": config.Consumer.Port,
			})
		}
	}()

	// Start listener for gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Consumer.GrpcPort))
	if err != nil {
		logger.LogError("Failed to listen", err, &logger.LogContext{
			"grpc_port": config.Consumer.GrpcPort,
		})
		return
	}

	// Start the pprof server
	go func() {
		logger.LogInfo("Starting pprof server", &logger.LogContext{
			"port": config.Consumer.ProfilingPort,
		})
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Consumer.ProfilingPort), nil); err != nil {
			logger.LogError("Failed to start pprof server", err, &logger.LogContext{
				"port": config.Consumer.ProfilingPort,
			})
		}
	}()

	// Set up DB connection
	dbSource := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Database.User,
		config.Database.Password,
		config.Database.Host,
		config.Database.Port,
		config.Database.DbName,
		config.Database.SslMode,
	)
	db, err := sql.Open("postgres", dbSource)
	if err != nil {
		logger.LogError("Failed to connect to the database", err, &logger.LogContext{
			"host": config.Database.Host,
			"port": config.Database.Port,
			"db":   config.Database.DbName,
		})
		return
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			logger.LogError("Error closing DB connection", err, &logger.LogContext{})
		}
	}(db)

	// Initialize sqlc queries struct
	queries := persistence.New(db)

	// Create a rate limiter: allows consumptions of tasks per second as set in config
	limiter := rate.NewLimiter(rate.Limit(config.RateLimiter.TasksPerSecond), 1)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterTaskServiceServer(grpcServer, &server{
		limiter: limiter,
		queries: queries, // Inject queries into the server
	})

	logger.LogInfo("Consumer service listening", &logger.LogContext{
		"grpc_port": config.Consumer.GrpcPort,
	})

	// Start the gRPC server
	if err := grpcServer.Serve(lis); err != nil {
		logger.LogError("Failed to serve gRPC server", err, &logger.LogContext{
			"grpc_port": config.Consumer.GrpcPort,
		})
	}
}

func (s *server) SendTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	rateLimitError := s.limiter.Wait(ctx)
	if rateLimitError != nil {
		logger.LogError("Rate limiter failed", rateLimitError, &logger.LogContext{})
	} else {
		logger.LogInfo("Processing task ....", &logger.LogContext{
			"task_id":    req.Id,
			"task_type":  req.Type,
			"task_value": req.Value,
		})
	}

	// Step 1: Update task state to "processing" immediately when received by the consumer
	err := s.updateTaskState(req.Id, "processing")
	if err != nil {
		taskProcessingFailures.Inc() // Increment the failure metric
		return nil, err
	}

	// Increment the "in processing" gauge
	tasksInProcessing.Inc()

	// Step 2: Simulate processing delay
	delayTime := time.Duration(req.Value) * time.Millisecond
	time.Sleep(delayTime)
	logger.LogInfo("Delaying task", &logger.LogContext{
		"task_id":    req.Id,
		"task_type":  req.Type,
		"task_value": req.Value,
		"delay":      delayTime,
	})

	// Step 3: Update task state to "done" after processing is complete
	err = s.updateTaskState(req.Id, "done")
	if err != nil {
		taskProcessingFailures.Inc() // Increment the failure metric
		return nil, err
	}

	// Decrement the "in processing" gauge as task is done
	tasksInProcessing.Dec()

	// Increment the processed tasks counter
	tasksProcessed.Inc()

	// Increment the processed tasks by type
	tasksByType.With(prometheus.Labels{"task_type": string(req.Type)}).Inc()

	// Add task value to the total sum for this task type in Prometheus
	taskValuesByType.With(prometheus.Labels{"task_type": string(req.Type)}).Add(float64(req.Value))

	// Update the local map with the total sum of task values for this task type
	taskTypeSumsMu.Lock()
	taskTypeSums[req.Type] += float64(req.Value)
	totalValueForType := taskTypeSums[req.Type]
	taskTypeSumsMu.Unlock()

	// Log the task details and total sum for the task type
	logger.LogInfo("Task processed", &logger.LogContext{
		"task_id":              req.Id,
		"task_type":            req.Type,
		"task_value":           req.Value,
		"total_value_for_type": totalValueForType,
	})

	return &pb.TaskResponse{Status: "Processed"}, nil
}

func (s *server) updateTaskState(taskID int32, state string) error {
	// Create context for the SQL query
	ctx := context.Background()

	// Create parameters for the UpdateTaskState query
	params := persistence.UpdateTaskStateParams{
		ID:    taskID,
		State: sql.NullString{String: state, Valid: true},
	}

	// Call the UpdateTaskState query using sqlc-generated function
	return s.queries.UpdateTaskState(ctx, params)
}
