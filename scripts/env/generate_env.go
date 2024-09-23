package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func main() {
	// Run `git describe --tags --always` to get the current Git tag or commit hash
	cmd := exec.Command("git", "describe", "--tags", "--always")
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to get Git version: %v", err)
	}

	// Trim the newline character from the output
	version := string(output)
	version = version[:len(version)-1] // Remove trailing newline

	// Define the path to the .env file in the parent directory
	envFilePath := ".env"

	// Open or create the .env file
	file, err := os.Create(envFilePath)
	if err != nil {
		log.Fatalf("Failed to create .env file: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	// Write the version to the .env file
	_, err = file.WriteString(fmt.Sprintf("VERSION=%s\n", version))
	if err != nil {
		log.Fatalf("Failed to write to .env file: %v", err)
	}

	fmt.Println("Successfully generated .env file with VERSION:", version)
}
