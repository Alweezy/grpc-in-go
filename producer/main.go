package main

import (
	"context"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof" // This import is necessary to initialize the pprof endpoints
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

// Configurable max backlog size (set a threshold for unprocessed tasks)
const maxBacklog = 100

// Track the actual backlog size in a local variable
var currentBacklog int

func init() {
	// Register the Prometheus metrics
	prometheus.MustRegister(tasksProduced)
	prometheus.MustRegister(taskProductionFailures)
	prometheus.MustRegister(backlogSize)
}

func main() {
	// Start the Prometheus metrics endpoint in a separate goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Prometheus metrics available at http://localhost:2112/metrics")
		log.Fatal(http.ListenAndServe(":2112", nil)) // Port for Prometheus metrics
	}()

	// Start the pprof server
	go func() {
		log.Println("Starting pprof server on :6060")
		log.Fatal(http.ListenAndServe(":6060", nil)) // Bind to all network interfaces
	}()

	// Initialize DB connection
	db, err := sql.Open("postgres", "postgresql://admin:password@db:5432/tasks_db?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Printf("Error closing DB connection: %v", err)
		}
	}(db)

	// Initialize Queries struct for SQL operations
	queries := persistence.New(db)

	// TODO: Alvin, if this were production ready, we need to establish gRPC secure conn
	// Establish gRPC connection
	// creds, err := credentials.NewClientTLSFromFile("path/to/cert/file", "")
	// conn, err := grpc.Dial("consumer:50051", grpc.WithTransportCredentials(creds))
	// Establish gRPC connection with insecure credentials (for development purposes)
	conn, err := grpc.Dial("consumer:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	//conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Consumer: %v", err)
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	// Simulate task production with controlled rate and backlog handling
	ticker := time.NewTicker(time.Second) // Produces one task every second
	defer ticker.Stop()

	for range ticker.C {
		if currentBacklog >= maxBacklog {
			log.Println("Max backlog reached, pausing task production...")
			continue // Skip task production until backlog decreases
		}

		taskType := rand.Intn(10)
		taskValue := rand.Intn(100)

		// Insert task into DB using sqlc-generated function
		err = createTask(queries, taskType, taskValue)
		if err != nil {
			taskProductionFailures.Inc() // Increment task production failure metric
			log.Printf("Failed to create task: %v", err)
			continue
		}

		// Increment the produced tasks counter
		tasksProduced.Inc()

		// Increment the local backlog size and the Prometheus gauge
		currentBacklog++
		backlogSize.Set(float64(currentBacklog))

		// Send task over gRPC to Consumer
		sendTask(client, taskType, taskValue)

		// Simulate task being removed from backlog after consumption
		// This simulates the idea of the task being consumed from the queue
		currentBacklog--
		backlogSize.Set(float64(currentBacklog))
	}
}

func createTask(queries *persistence.Queries, taskType int, taskValue int) error {
	ctx := context.Background()

	// Populate CreateTaskParams
	params := persistence.CreateTaskParams{
		Type:  sql.NullInt32{Int32: int32(taskType), Valid: true},
		Value: sql.NullInt32{Int32: int32(taskValue), Valid: true},
	}

	// Insert task into DB
	return queries.CreateTask(ctx, params)
}

func sendTask(client pb.TaskServiceClient, taskType int, taskValue int) {
	_, err := client.SendTask(context.Background(), &pb.TaskRequest{
		Type:  int32(taskType),
		Value: int32(taskValue),
	})
	if err != nil {
		log.Printf("Failed to send task: %v", err)
	}
}
