package main

import (
	"context"
	"database/sql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"log"
	"net"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
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
)

func init() {
	// Register the Prometheus metrics
	prometheus.MustRegister(tasksProcessed)
	prometheus.MustRegister(taskProcessingFailures)
}

type server struct {
	pb.UnimplementedTaskServiceServer
	limiter *rate.Limiter
	queries *persistence.Queries
}

func (s *server) SendTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	// Wait for permission to process a new task based on rate limit
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	log.Printf("Processing task: Type=%d, Value=%d", req.Type, req.Value)

	// Simulate processing delay
	time.Sleep(time.Duration(req.Value) * time.Millisecond)

	// Update task state to "done"
	err := s.updateTaskState(req.Type, "done") // UpdateTaskState logic
	if err != nil {
		taskProcessingFailures.Inc() // Increment the failure metric
		return nil, err
	}

	tasksProcessed.Inc() // Increment the processed tasks counter
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

func main() {
	// Start the Prometheus metrics endpoint in a separate goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Prometheus metrics available at http://localhost:2113/metrics")
		log.Fatal(http.ListenAndServe(":2113", nil)) // Port for Prometheus metrics
	}()

	// Start listener for gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Set up DB connection
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

	log.Println("Consumer service listening on :50051")

	// Start the gRPC server
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
