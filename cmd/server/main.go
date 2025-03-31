package main

// TODO: Refactor main.go to separate server initialization logic from configuration loading
// TODO: Create dedicated server setup package for better separation of concerns
// TODO: Implement structured logging instead of using standard log package
// TODO: Consider using a configuration management library like Viper for better config handling

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/FreePeak/cortex/pkg/server"

	"github.com/FreePeak/db-mcp-server/internal/config"
	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
	"github.com/FreePeak/db-mcp-server/internal/repository"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// findConfigFile attempts to find config.json in the current directory or parent directories
func findConfigFile() string {
	// Default config file name
	const defaultConfigFile = "config.json"

	// Check if the file exists in current directory
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current directory: %v", err)
		return defaultConfigFile
	}

	// Try up to 3 parent directories
	for i := 0; i < 3; i++ {
		cwd = filepath.Dir(cwd)
		configPath := filepath.Join(cwd, defaultConfigFile)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Fall back to default if not found
	return defaultConfigFile
}

func main() {
	// Parse command-line arguments
	configFile := flag.String("c", "config.json", "Database configuration file")
	configPath := flag.String("config", "config.json", "Database configuration file (alternative)")
	transportMode := flag.String("t", "sse", "Transport mode (stdio or sse)")
	serverPort := flag.Int("p", 9092, "Server port for SSE transport")
	serverHost := flag.String("h", "localhost", "Server host for SSE transport")
	dbConfigJSON := flag.String("db-config", "", "JSON string with database configuration")
	flag.Parse()

	// Prioritize flags with actual values
	finalConfigPath := *configFile
	if finalConfigPath == "config.json" && *configPath != "config.json" {
		finalConfigPath = *configPath
	}

	// If no specific config path was provided, try to find a config file
	if finalConfigPath == "config.json" {
		possibleConfigPath := findConfigFile()
		if possibleConfigPath != "config.json" {
			log.Printf("Found config file at: %s", possibleConfigPath)
			finalConfigPath = possibleConfigPath
		}
	}

	finalServerPort := *serverPort
	// Set environment variables from command line arguments if provided
	if finalConfigPath != "config.json" {
		if err := os.Setenv("CONFIG_PATH", finalConfigPath); err != nil {
			log.Printf("Warning: failed to set CONFIG_PATH env: %v", err)
		}
	}
	if *transportMode != "sse" {
		if err := os.Setenv("TRANSPORT_MODE", *transportMode); err != nil {
			log.Printf("Warning: failed to set TRANSPORT_MODE env: %v", err)
		}
	}
	if finalServerPort != 9092 {
		if err := os.Setenv("SERVER_PORT", fmt.Sprintf("%d", finalServerPort)); err != nil {
			log.Printf("Warning: failed to set SERVER_PORT env: %v", err)
		}
	}
	// Set DB_CONFIG environment variable if provided via flag
	if *dbConfigJSON != "" {
		if err := os.Setenv("DB_CONFIG", *dbConfigJSON); err != nil {
			log.Printf("Warning: failed to set DB_CONFIG env: %v", err)
		}
	}

	// Load configuration after environment variables are set
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Warning: Failed to load configuration: %v", err)
		// Create a default config if loading fails
		cfg = &config.Config{
			ServerPort:    finalServerPort,
			TransportMode: *transportMode,
			ConfigPath:    finalConfigPath,
		}
	}

	// Initialize database connection from config
	dbConfig := &dbtools.Config{
		ConfigFile: cfg.ConfigPath,
	}

	// Ensure database configuration exists
	log.Printf("Using database configuration from: %s", cfg.ConfigPath)

	// Try to initialize database from config
	if err := dbtools.InitDatabase(dbConfig); err != nil {
		log.Printf("Warning: Failed to initialize database: %v", err)
	}

	// Set up signal handling for clean shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create logger for the MCP server
	logger := log.New(os.Stderr, "[DB MCP Server] ", log.LstdFlags)

	// Create mcp-go server
	mcpServer := server.NewMCPServer(
		"DB MCP Server", // Server name
		"1.0.0",         // Server version
		logger,          // Logger
	)

	// Set up Clean Architecture layers
	dbRepo := repository.NewDatabaseRepository()
	dbUseCase := usecase.NewDatabaseUseCase(dbRepo)
	toolRegistry := mcp.NewToolRegistry(mcpServer, cfg.ServerName)

	// Set the database use case in the tool registry
	ctx := context.Background()

	// Debug log: Check database connections before registering tools
	dbIDs := dbUseCase.ListDatabases()
	if len(dbIDs) > 0 {
		log.Printf("Detected %d database connections: %v", len(dbIDs), dbIDs)
		log.Printf("Will dynamically generate database tools for each connection")
	} else {
		log.Printf("No database connections detected")
	}

	// Register tools
	if err := toolRegistry.RegisterAllTools(ctx, dbUseCase); err != nil {
		log.Printf("Warning: error registering tools: %v", err)
	}
	log.Printf("Finished registering tools")

	// If we have databases, display the available tools
	if len(dbIDs) > 0 {
		// Calculate tool prefix for display
		toolPrefix := fmt.Sprintf("mcp_%s_", cfg.ServerName)

		log.Printf("Available database tools:")
		for _, dbID := range dbIDs {
			log.Printf("  Database %s:", dbID)
			log.Printf("    - %squery_%s: Execute SQL queries", toolPrefix, dbID)
			log.Printf("    - %sexecute_%s: Execute SQL statements", toolPrefix, dbID)
			log.Printf("    - %stransaction_%s: Manage transactions", toolPrefix, dbID)
			log.Printf("    - %sperformance_%s: Analyze query performance", toolPrefix, dbID)
			log.Printf("    - %sschema_%s: Get database schema", toolPrefix, dbID)
		}
		log.Printf("  Common tools:")
		log.Printf("    - %slist_databases: List all available databases", toolPrefix)
	}

	// If no database connections, register mock tools to ensure at least some tools are available
	if len(dbIDs) == 0 {
		log.Printf("No database connections available. Adding mock tools...")
		if err := toolRegistry.RegisterMockTools(ctx); err != nil {
			log.Printf("Warning: error registering mock tools: %v", err)
		}
	}

	// Create a session store to track valid sessions
	sessions := make(map[string]bool)

	// Create a default session for easier testing
	defaultSessionID := "default-session"
	sessions[defaultSessionID] = true
	log.Printf("Created default session: %s", defaultSessionID)

	// Handle transport mode
	switch cfg.TransportMode {
	case "sse":
		log.Printf("Starting SSE server on port %d", cfg.ServerPort)

		// Configure base URL with explicit protocol
		baseURL := fmt.Sprintf("http://%s:%d", *serverHost, cfg.ServerPort)
		log.Printf("Using base URL: %s", baseURL)

		// Set logging mode based on configuration
		if cfg.DisableLogging {
			log.Printf("Logging in SSE transport is disabled")
			// Redirect standard output to null device if logging is disabled
			// This only works on Unix-like systems
			if err := os.Setenv("MCP_DISABLE_LOGGING", "true"); err != nil {
				log.Printf("Warning: failed to set MCP_DISABLE_LOGGING env: %v", err)
			}
		}
		// Set the server address
		mcpServer.SetAddress(fmt.Sprintf(":%d", cfg.ServerPort))

		// Start the server
		errCh := make(chan error, 1)
		go func() {
			log.Printf("Starting server...")
			errCh <- mcpServer.ServeHTTP()
		}()

		// Wait for interrupt or error
		select {
		case err := <-errCh:
			log.Fatalf("Server error: %v", err)
		case <-stop:
			log.Println("Shutting down server...")

			// Create shutdown context
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			// Shutdown the server
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("Error during server shutdown: %v", err)
			}

			// Close database connections
			if err := dbtools.CloseDatabase(); err != nil {
				log.Printf("Error closing database connections: %v", err)
			}
		}

	case "stdio":
		log.Printf("Starting STDIO server")

		// Set logging mode based on configuration
		if cfg.DisableLogging {
			log.Printf("Logging in STDIO transport is disabled")
			// Set environment variable to signal to the MCP library to disable logging
			if err := os.Setenv("MCP_DISABLE_LOGGING", "true"); err != nil {
				log.Printf("Warning: failed to set MCP_DISABLE_LOGGING env: %v", err)
			}
		}

		// No graceful shutdown needed for stdio
		if err := mcpServer.ServeStdio(); err != nil {
			log.Fatalf("STDIO server error: %v", err)
		}

	default:
		log.Fatalf("Invalid transport mode: %s", cfg.TransportMode)
	}

	log.Println("Server shutdown complete")
}
