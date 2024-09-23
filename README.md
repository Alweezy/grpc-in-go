# Task Processing System

This project implements a **task producer-consumer** system where tasks are created by a producer, processed by a consumer, and monitored using **Prometheus** with visualizations in **Grafana**. The project provides a **fully Dockerized setup** (recommended) as well as a **local setup with a Dockerized database**.

---

## Project Structure

```bash
grpc-in-go/
├── consumer/             # Consumer service for processing tasks
│   └── main.go
├── producer/             # Producer service for creating tasks
│   └── main.go
├── migrations/           # SQL migration files
│   └── 000001_create_tasks_table.up.sql
├── pb/                   # Protocol Buffers (generated)
│   └── tasks.pb.go
├── scripts/              # Custom scripts (e.g., for migrations)
│   └── migrate.go        # Script to run migrations
├── Dockerfile.producer   # Dockerfile for Producer
├── Dockerfile.consumer   # Dockerfile for Consumer
├── docker-compose.yml    # Docker Compose configuration
├── prometheus.yml        # Prometheus configuration file
├── README.md             # Project documentation
└── go.mod                # Go module definition
```

## 1. Project Setup

### Dockerized Setup (Recommended)

This setup runs all services—Producer, Consumer, PostgreSQL, Prometheus, and Grafana—in Docker containers using Docker Compose.

#### Steps:
#### 1. Clone this repository
```
git clone git@github.com:Alweezy/grpc-in-go.git
cd grpc-in-go
```

#### 2.	Run the Dockerized environment:
As a preliminary, run:
```
go run scripts/env/generate_env.go
```
> 💡 This script execution will be necessary to obtain the version with which you are building your services with
```
docker-compose up --build
```
#### This will:
> Start PostgreSQL on port 15432.
> 
> Start Producer on port 2112.
> 
> Start Consumer on port 2113.
> 
> Start Prometheus on port 9090.
> 
> Start Grafana on port 3000.

#### 3.	Access services:
•	Grafana: Go to http://localhost:3000 (default credentials: admin/admin).

•	Prometheus: Go to http://localhost:9090.

•	Producer metrics: http://localhost:2112/metrics.

•	Consumer metrics: http://localhost:2113/metrics.


### Local Setup (Running Producer/Consumer Locally)

If you prefer to run Producer and Consumer services locally while still using Docker for the PostgreSQL database, Prometheus, and Grafana, follow these steps:

#### Steps:

#### 1.	Start the database and monitoring stack:

```
docker-compose up db prometheus grafana
```
This starts PostgreSQL, Prometheus, and Grafana containers.

#### 2.	Set up the Go environment:
Ensure that you have Go installed locally.
#### 3.	Run migrations:
Before starting the services, you need to run the database migrations locally. Run the following command:

```
go run scripts/migrate.go migrations/000001_create_tasks_table.up.sql
```
#### 4.	Run Producer and Consumer services locally:
In separate terminal windows, run the following commands:
###### Producer:

```
export ENVIRONMENT=local
cd producer
go run main.go
```
###### Consumer:
```
export ENVIRONMENT=local
cd consumer
go run main.go
```

#### 5.	Access services:
•	Producer metrics available at http://localhost:2112/metrics.

•	Consumer metrics available at http://localhost:2113/metrics.

•	Grafana: Go to http://localhost:3000 (default credentials: admin/admin).

•	Prometheus: Go to http://localhost:9090.

### 2. Database Migrations

#### Applying Migrations

The migration files are stored in the migrations/ directory. The provided Go script scripts/migrate/migrate.go can be used to apply migrations.

Steps:

#### 1.	Apply a migration:
On a terminal session un the following command, replacing <migration_file> with the actual migration file (e.g., 000001_create_tasks_table.up.sql):

```
export ENVIRONMENT=local

go run scripts/migrate/migrate.go migrations/000001_create_tasks_table.up.sql>
```
This will apply the migration to the PostgreSQL database running inside the Docker container.

### 3. Running Tests with Coverage

This project includes unit tests for the core logic and SQL queries.

#### Running Tests

To run the tests with coverage reporting, execute the following command:

```
go test ./... -cover
```
This will run all tests and output the coverage percentage for each package.

To generate a detailed HTML coverage report, use:

```
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

This will open an interactive coverage report in your browser.

### 4. Monitoring and Visualization

The system exposes Prometheus metrics from both the Producer and Consumer services, which are visualized in Grafana.

Prometheus

•	Prometheus scrapes metrics from the Producer and Consumer services every few seconds.

•	Prometheus is available at http://localhost:9090.

#### Grafana

•	Grafana is available at http://localhost:3000.
•	Default credentials: admin/admin.

> 👉 Grafana might prompt you to change the password but that's pretty straightforward!

You can set up dashboards in Grafana to visualize the following:

•	Number of tasks in each state (received, processing, done).

•	Total sum of task values processed by the consumer.

•	Status of services (up or down).

### 5. Customization

Configuring Task Rate Limiting

The Consumer service uses a rate limiter to control the number of tasks processed per second. You can configure the rate limiter by adjusting the limiter initialization in consumer/main.go:

```
limiter := rate.NewLimiter(5, 1) // Allows 5 tasks per second

```
Change the parameters to modify the rate limit. (This can be done in the configs/consumer*)

```
rate_limiter:
  tasks_per_second: 5
```

