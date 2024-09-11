// hint: to create a migrations file use: migrate create -ext sql -dir migrations -seq <migrations_migrations_file_name>
// Add migrations to the generated up and down pair of files

package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"os"
)

func main() {
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
	db, err := sql.Open("postgres", "postgres://admin:password@localhost:15432/tasks_db?sslmode=disable")
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
