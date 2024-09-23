package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"grpc-in-go/util"
	"grpc-in-go/util/logger"
	"math/rand"
	"net/http"
	_ "net/http/pprof" // This import is necessary to initialize the pprof endpoints
	"os"
	"time"
)

// Prometheus metrics
var (
	tasksProduced = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tasks_produced_total",
		Help: "Total number of tasks produced by the producer service",
	})
	taskProductionFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "task_production_failures_total",
		Help: "Total number of task production failures",
	})
	backlogSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "task_producer_backlog_size",
		Help: "Current number of unprocessed tasks in the backlog",
	})
)

// Config struct to hold configuration values
type Config struct {
	Database    Database          `mapstructure:"database"`
	Producer    Producer          `mapstructure:"producer"`
	Prometheus  Prometheus        `mapstructure:"prometheus"`
	Logger      *logger.LogConfig `mapstructure:"logger"`
	RateLimiter RateLimiter       `mapstructure:"rate_limiter"`
	MaxBackLog  int               `mapstructure:"max_backlog"`
}

type Database struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DbName   string `mapstructure:"dbname"`
	SslMode  string `mapstructure:"sslmode"`
}

type Producer struct {
	Port            int    `mapstructure:"port"`
	ProfilingPort   int    `mapstructure:"profiling_port"`
	GrpcConsumerUrl string `mapstructure:"grpc_consumer_url"`
}

type Prometheus struct {
	ScrapeInterval string `mapstructure:"scrape_interval"`
}

type RateLimiter struct {
	TickerTime float64 `mapstructure:"ticker_time"`
}

var currentBacklog int

func init() {
	// Register the Prometheus metrics
	prometheus.MustRegister(tasksProduced)
	prometheus.MustRegister(taskProductionFailures)
	prometheus.MustRegister(backlogSize)
}

var version string

func main() {
	// Check if version flag is provided
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Printf("Producer service version: %s\n", version)
		return
	}

	// Define the AppConfig to load the producer config file
	appConfig := &util.AppConfig{
		FilePath: "configs",
		FileName: "producer",
		Type:     util.ConfigYAML,
	}

	// Define a Config struct to hold the loaded configuration
	var config Config

	// Load the configuration using the utility function
	if err := util.LoadConfig(appConfig, &config); err != nil {
		logger.LogError("Failed to load configuration", err, &logger.LogContext{
			"service": "producer",
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
			"service": "producer",
		})
		return
	}

	logger.LogInfo("Producer service starting", &logger.LogContext{
		"version": version,
	})

	// Start the Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		logger.LogInfo("Prometheus metrics available", &logger.LogContext{
			"port": config.Producer.Port,
			"url":  fmt.Sprintf("http://localhost:%d/metrics", config.Producer.Port),
		})
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Producer.Port), nil); err != nil {
			logger.LogError("Failed to start Prometheus metrics server", err, &logger.LogContext{
				"port": config.Producer.Port,
			})
		}
	}()

	// Start the pprof server on the configured profiling port
	go func() {
		logger.LogInfo("Starting pprof server", &logger.LogContext{
			"port": config.Producer.ProfilingPort,
		})
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Producer.ProfilingPort), nil); err != nil {
			logger.LogError("Failed to start pprof server", err, &logger.LogContext{
				"port": config.Producer.ProfilingPort,
			})
		}
	}()

	// Initialize DB connection using the config values
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
	defer db.Close()

	queries := persistence.New(db)

	// Establish gRPC connection with the consumer
	conn, err := grpc.Dial(config.Producer.GrpcConsumerUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.LogError("Failed to connect to Consumer", err, &logger.LogContext{
			"grpc_url": config.Producer.GrpcConsumerUrl,
		})
		return
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	// Simulate task production with controlled rate and backlog handling
	// produce one task every 500 microseconds
	ticker := time.NewTicker(time.Duration(config.RateLimiter.TickerTime) * time.Microsecond)
	defer ticker.Stop()

	maxBacklog := config.MaxBackLog
	for range ticker.C {
		if currentBacklog >= maxBacklog {
			logger.LogWarn("Max backlog reached, pausing task production", &logger.LogContext{
				"backlog_size": currentBacklog,
			})
			continue
		}

		taskType := rand.Intn(10)
		taskValue := rand.Intn(100)

		taskID, err := createTask(queries, taskType, taskValue)
		if err != nil {
			taskProductionFailures.Inc()
			logger.LogError("Failed to create task", err, &logger.LogContext{
				"task_type":  taskType,
				"task_value": taskValue,
			})
			continue
		}

		tasksProduced.Inc()
		currentBacklog++
		backlogSize.Set(float64(currentBacklog))
		logger.LogInfo(fmt.Sprintf("Current backlog size: %d", currentBacklog), &logger.LogContext{
			"backlog_size": currentBacklog,
		})

		sendTask(client, taskID, taskType, taskValue)
	}
}

func createTask(queries *persistence.Queries, taskType int, taskValue int) (int32, error) {
	ctx := context.Background()

	params := persistence.CreateTaskParams{
		Type:  sql.NullInt32{Int32: int32(taskType), Valid: true},
		Value: sql.NullInt32{Int32: int32(taskValue), Valid: true},
	}

	// Create task and return the generated ID
	taskID, err := queries.CreateTask(ctx, params)
	if err != nil {
		return 0, err
	}

	logger.LogInfo("Task created", &logger.LogContext{
		"task_id":    taskID,
		"task_type":  taskType,
		"task_value": taskValue,
	})

	return taskID, nil
}

func sendTask(client pb.TaskServiceClient, taskID int32, taskType int, taskValue int) {
	go func() {
		_, err := client.SendTask(context.Background(), &pb.TaskRequest{
			Id:    taskID,
			Type:  int32(taskType),
			Value: int32(taskValue),
		})
		if err != nil {
			logger.LogError("Failed to send task", err, &logger.LogContext{
				"task_id":    taskID,
				"task_type":  taskType,
				"task_value": taskValue,
			})
		} else {
			// Only decrement backlog when the consumer successfully processes the task
			currentBacklog--
			backlogSize.Set(float64(currentBacklog))
			logger.LogInfo("Task processed, backlog decremented", &logger.LogContext{
				"task_id": taskID,
				"backlog": currentBacklog,
			})
		}
	}()
}
