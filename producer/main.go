package main

import (
	"context"
	"database/sql"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"math/rand"
	"time"
	"yq-app-challenge/pb"
	"yq-app-challenge/persistence"
)

func main() {
	// Initialize DB connection
	//db, err := sql.Open("postgres", "postgresql://admin:password@db:5432/tasks_db?sslmode=disable")
	//if err != nil {
	//	log.Fatalf("Failed to connect to the database: %v", err)
	//}
	// Change "db" to "localhost" since you are running the producer locally
	db, err := sql.Open("postgres", "postgresql://admin:password@localhost:5432/tasks_db?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Initialize Queries struct for SQL operations
	queries := persistence.New(db)

	// TODO: Alvin, if this were production ready, we need to establish gRPC secure conn
	// Establish gRPC connection
	//creds, err := credentials.NewClientTLSFromFile("path/to/cert/file", "")
	//conn, err := grpc.Dial("consumer:50051", grpc.WithTransportCredentials(creds))
	// Establish gRPC connection with insecure credentials (for development purposes)
	//conn, err := grpc.Dial("consumer:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Consumer: %v", err)
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	// Simulate task production
	for {
		taskType := rand.Intn(10)
		taskValue := rand.Intn(100)

		// Insert task into DB using sqlc-generated function
		err = createTask(queries, taskType, taskValue)
		if err != nil {
			log.Fatalf("Failed to create task: %v", err)
		}

		// Send task over gRPC to Consumer
		sendTask(client, taskType, taskValue)

		// Control message production rate
		time.Sleep(time.Second * 1)
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
