package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"grpc-in-go/pb"
	"grpc-in-go/persistence"
	"grpc-in-go/util"
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

// Config struct to hold configuration values
type Config struct {
	Database   Database   `mapstructure:"database"`
	Producer   Producer   `mapstructure:"producer"`
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

type Producer struct {
	Port            int    `mapstructure:"port"`
	ProfilingPort   int    `mapstructure:"profiling_port"`
	GrpcConsumerUrl string `mapstructure:"grpc_consumer_url"`
}

type Prometheus struct {
	ScrapeInterval string `mapstructure:"scrape_interval"`
}

const maxBacklog = 100

var currentBacklog int

func init() {
	// Register the Prometheus metrics
	prometheus.MustRegister(tasksProduced)
	prometheus.MustRegister(taskProductionFailures)
	prometheus.MustRegister(backlogSize)
}

func main() {
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
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Start the Prometheus metrics endpoint
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Prometheus metrics available at http://localhost:%d/metrics", config.Producer.Port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Producer.Port), nil))
	}()

	// Start the pprof server on the configured profiling port
	go func() {
		log.Printf("Starting pprof server on :%d", config.Producer.ProfilingPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.Producer.ProfilingPort), nil))
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
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer db.Close()

	queries := persistence.New(db)

	// Establish gRPC connection with the consumer using the config value
	conn, err := grpc.Dial(config.Producer.GrpcConsumerUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Consumer: %v", err)
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)

	// Simulate task production with controlled rate and backlog handling
	ticker := time.NewTicker(time.Millisecond) // Produces one task every second
	defer ticker.Stop()

	for range ticker.C {
		if currentBacklog >= maxBacklog {
			log.Println("Max backlog reached, pausing task production...")
			continue
		}

		taskType := rand.Intn(10)
		taskValue := rand.Intn(100)

		taskID, err := createTask(queries, taskType, taskValue)
		if err != nil {
			taskProductionFailures.Inc()
			log.Printf("Failed to create task: %v", err)
			continue
		}

		tasksProduced.Inc()
		currentBacklog++
		backlogSize.Set(float64(currentBacklog))

		sendTask(client, taskID, taskType, taskValue)

		currentBacklog--
		backlogSize.Set(float64(currentBacklog))
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

	return taskID, nil
}

func sendTask(client pb.TaskServiceClient, taskID int32, taskType int, taskValue int) {
	_, err := client.SendTask(context.Background(), &pb.TaskRequest{
		Id:    taskID, // Send the task ID
		Type:  int32(taskType),
		Value: int32(taskValue),
	})
	if err != nil {
		log.Printf("Failed to send task: %v", err)
	}
}
