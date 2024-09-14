// hint: to create a migrations file use: migrate create -ext sql -dir migrations -seq <migrations_migrations_file_name>
// Add migrations to the generated up and down pair of files

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"grpc-in-go/util"
	"io/ioutil"
	"log"
	"os"
)

type Config struct {
	Database Database `mapstructure:"database"`
}

type Database struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DbName   string `mapstructure:"dbname"`
	SslMode  string `mapstructure:"sslmode"`
}

func main() {
	appConfig := &util.AppConfig{
		FilePath: "configs",
		FileName: "consumer",
		Type:     util.ConfigYAML,
	}

	var config Config

	// Load the configuration using the utility function
	if err := util.LoadConfig(appConfig, &config); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dbSource := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Database.User,
		config.Database.Password,
		config.Database.Host,
		config.Database.Port,
		config.Database.DbName,
		config.Database.SslMode,
	)

	// Accept migration file as an argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run migrate.go <migration_file.sql>")
		os.Exit(1)
	}

	migrationFile := os.Args[1]

	// Read the SQL file
	sqlBytes, err := ioutil.ReadFile(fmt.Sprintf("migrations/%s", migrationFile))
	if err != nil {
		log.Fatalf("Failed to read the migration file %s: %v", migrationFile, err)
	}
	sqlQuery := string(sqlBytes)

	// Connect to the Postgres database (adjust connection string as needed)
	db, err := sql.Open("postgres", dbSource)
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Run the SQL query (migration)
	_, err = db.Exec(sqlQuery)
	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	log.Printf("Migration %s applied successfully!", migrationFile)
}
