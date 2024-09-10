package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"time"
	"yq-app-challenge/pb"
	"yq-app-challenge/persistence"

	_ "github.com/lib/pq"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

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
		return nil, err
	}

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
	// Start listener for gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Set up DB connection
	db, err := sql.Open("postgres", "postgresql://admin:password@localhost:5432/tasks_db?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

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
