package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"grpc-in-go/util"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
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
	Database   Database   `mapstructure:"database"`
	Consumer   Consumer   `mapstructure:"consumer"`
	Prometheus Prometheus `mapstructure:"prometheus"`
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

func main() {

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
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Start the Prometheus metrics endpoint in a separate goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Prometheus metrics available at http://localhost:%d/metrics", config.Consumer.Port)
		log.Fatal(http.ListenAndServe(":2113", nil)) // Port for Prometheus metrics
	}()

	// Start listener for gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Consumer.GrpcPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Start the pprof server
	go func() {
		log.Printf("Starting pprof server on :%d", config.Consumer.ProfilingPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Consumer.ProfilingPort), nil))
	}()

	// Set up DB connection
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
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("Error closing DB connection: %v", err)
		}
	}(db)

	// Initialize sqlc queries struct
	queries := persistence.New(db)

	// Create a rate limiter: allows 5 tasks per second
	limiter := rate.NewLimiter(5, 1)

	// Initialize gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterTaskServiceServer(grpcServer, &server{
		limiter: limiter,
		queries: queries, // Inject queries into the server
	})

	log.Printf("Consumer service listening on :%d", config.Consumer.GrpcPort)

	// Start the gRPC server
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

func (s *server) SendTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	// Wait for permission to process a new task based on rate limit
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	log.Printf("Processing task ID: %d, Type=%d, Value=%d", req.Id, req.Type, req.Value)

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
	log.Printf("Delaying task ID: %d: Type=%d, Value=%d", req.Id, req.Type, req.Value)
	log.Printf("Task ID: %d delayed by: %v", req.Id, delayTime)

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
	log.Printf("Task, Task ID: %d processed (Done): Type=%d, Value=%d, TotalValueForType=%f", req.Id, req.Type, req.Value, totalValueForType)

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
