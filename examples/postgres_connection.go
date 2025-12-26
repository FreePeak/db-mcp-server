// Package main provides examples of connecting to PostgreSQL databases using the db-mcp-server library.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func main() {
	// Example 1: Direct PostgreSQL 17 connection
	connectDirectly()

	// Example 2: Using the DB Manager with configuration file
	connectWithManager()
}

func connectDirectly() {
	fmt.Println("=== Example 1: Direct PostgreSQL 17 Connection ===")

	// Create configuration for PostgreSQL 17
	config := db.Config{
		Type:     "postgres",
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     5432,
		User:     getEnv("POSTGRES_USER", "postgres"),
		Password: getEnv("POSTGRES_PASSWORD", "postgres"),
		Name:     getEnv("POSTGRES_DB", "postgres"),

		// PostgreSQL 17 specific options
		SSLMode:            db.SSLPrefer,
		ApplicationName:    "db-mcp-example",
		ConnectTimeout:     10,
		TargetSessionAttrs: "any", // Works with PostgreSQL 10+

		// Additional options
		Options: map[string]string{
			"client_encoding": "UTF8",
		},

		// Connection pool settings
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}

	// Create database connection
	database, err := db.NewDatabase(config)
	if err != nil {
		log.Fatalf("Failed to create database instance: %v", err)
	}

	// Connect to the database
	fmt.Println("Connecting to PostgreSQL...")
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = database.Close() }()

	fmt.Println("Successfully connected to PostgreSQL")
	fmt.Println("Connection string (masked): ", database.ConnectionString())

	// Query PostgreSQL version to verify compatibility
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	err = database.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("Failed to query PostgreSQL version: %v", err)
	}

	fmt.Printf("Connected to: %s\n", version)

	// Run a sample query with PostgreSQL-style placeholders
	rows, err := database.Query(ctx, "SELECT datname FROM pg_database WHERE datistemplate = $1", false)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	fmt.Println("\nAvailable databases:")
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		fmt.Printf("- %s\n", dbName)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error during row iteration: %v", err)
	}

	fmt.Println()
}

func connectWithManager() {
	fmt.Println("=== Example 2: Using DB Manager with Configuration ===")

	// Create a database manager
	manager := db.NewDBManager()

	// Create sample configuration with PostgreSQL 17 settings
	config := []byte(`{
		"connections": [
			{
				"id": "postgres17",
				"type": "postgres",
				"host": "localhost", 
				"port": 5432,
				"name": "postgres",
				"user": "postgres",
				"password": "postgres",
				"ssl_mode": "prefer",
				"application_name": "db-mcp-example",
				"connect_timeout": 10,
				"target_session_attrs": "any",
				"options": {
					"client_encoding": "UTF8"
				},
				"max_open_conns": 10,
				"max_idle_conns": 5,
				"conn_max_lifetime_seconds": 300,
				"conn_max_idle_time_seconds": 60
			}
		]
	}`)

	// Update with environment variables
	// In a real application, you would load this from a file
	// and use proper environment variable substitution

	// Load configuration
	if err := manager.LoadConfig(config); err != nil {
		log.Fatalf("Failed to load database config: %v", err)
	}

	// Connect to databases
	fmt.Println("Connecting to all configured databases...")
	if err := manager.Connect(); err != nil {
		log.Fatalf("Failed to connect to databases: %v", err)
	}
	defer func() { _ = manager.CloseAll() }()

	// Get a specific database connection
	database, err := manager.GetDatabase("postgres17")
	if err != nil {
		log.Fatalf("Failed to get database: %v", err)
	}

	fmt.Println("Successfully connected to PostgreSQL via manager")
	fmt.Println("Connection string (masked): ", database.ConnectionString())

	// Query PostgreSQL version to verify compatibility
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	err = database.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("Failed to query PostgreSQL version: %v", err)
	}

	fmt.Printf("Connected to: %s\n", version)
	fmt.Println()
}

// Helper function to get environment variable with fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
