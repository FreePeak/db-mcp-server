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
	"github.com/FreePeak/db-mcp-server/internal/logger"
	"github.com/FreePeak/db-mcp-server/internal/repository"
	"github.com/FreePeak/db-mcp-server/internal/usecase"
	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
	pkgLogger "github.com/FreePeak/db-mcp-server/pkg/logger"
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
		logger.Error("Error getting current directory: %v", err)
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
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logger
	logger.Initialize(*logLevel)
	pkgLogger.Initialize(*logLevel)

	// Prioritize flags with actual values
	finalConfigPath := *configFile
	if finalConfigPath == "config.json" && *configPath != "config.json" {
		finalConfigPath = *configPath
	}

	// If no specific config path was provided, try to find a config file
	if finalConfigPath == "config.json" {
		possibleConfigPath := findConfigFile()
		if possibleConfigPath != "config.json" {
			logger.Info("Found config file at: %s", possibleConfigPath)
			finalConfigPath = possibleConfigPath
		}
	}

	finalServerPort := *serverPort
	// Set environment variables from command line arguments if provided
	if finalConfigPath != "config.json" {
		if err := os.Setenv("CONFIG_PATH", finalConfigPath); err != nil {
			logger.Warn("Warning: failed to set CONFIG_PATH env: %v", err)
		}
	}
	if *transportMode != "sse" {
		if err := os.Setenv("TRANSPORT_MODE", *transportMode); err != nil {
			logger.Warn("Warning: failed to set TRANSPORT_MODE env: %v", err)
		}
	}
	if finalServerPort != 9092 {
		if err := os.Setenv("SERVER_PORT", fmt.Sprintf("%d", finalServerPort)); err != nil {
			logger.Warn("Warning: failed to set SERVER_PORT env: %v", err)
		}
	}
	// Set DB_CONFIG environment variable if provided via flag
	if *dbConfigJSON != "" {
		if err := os.Setenv("DB_CONFIG", *dbConfigJSON); err != nil {
			logger.Warn("Warning: failed to set DB_CONFIG env: %v", err)
		}
	}

	// Load configuration after environment variables are set
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Warn("Warning: Failed to load configuration: %v", err)
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
	logger.Info("Using database configuration from: %s", cfg.ConfigPath)

	// Try to initialize database from config
	if err := dbtools.InitDatabase(dbConfig); err != nil {
		logger.Warn("Warning: Failed to initialize database: %v", err)
	}

	// Set up signal handling for clean shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Create mcp-go server with our logger's standard logger (compatibility layer)
	mcpServer := server.NewMCPServer(
		"DB MCP Server", // Server name
		"1.0.0",         // Server version
		nil,             // Use default logger
	)

	// Set up Clean Architecture layers
	dbRepo := repository.NewDatabaseRepository()
	dbUseCase := usecase.NewDatabaseUseCase(dbRepo)
	toolRegistry := mcp.NewToolRegistry(mcpServer)

	// Set the database use case in the tool registry
	ctx := context.Background()

	// Debug log: Check database connections before registering tools
	dbIDs := dbUseCase.ListDatabases()
	if len(dbIDs) > 0 {
		logger.Info("Detected %d database connections: %v", len(dbIDs), dbIDs)
		logger.Info("Will dynamically generate database tools for each connection")
	} else {
		logger.Info("No database connections detected")
	}

	// Register tools
	if err := toolRegistry.RegisterAllTools(ctx, dbUseCase); err != nil {
		logger.Warn("Warning: error registering tools: %v", err)
		// If there was an error registering tools, register mock tools as fallback
		logger.Info("Registering mock tools as fallback due to error...")
		if err := toolRegistry.RegisterMockTools(ctx); err != nil {
			logger.Warn("Warning: error registering mock tools: %v", err)
		}
	}
	logger.Info("Finished registering tools")

	// If we have databases, display the available tools
	if len(dbIDs) > 0 {
		logger.Info("Available database tools:")
		for _, dbID := range dbIDs {
			logger.Info("  Database %s:", dbID)
			logger.Info("    - query_%s: Execute SQL queries", dbID)
			logger.Info("    - execute_%s: Execute SQL statements", dbID)
			logger.Info("    - transaction_%s: Manage transactions", dbID)
			logger.Info("    - performance_%s: Analyze query performance", dbID)
			logger.Info("    - schema_%s: Get database schema", dbID)
		}
		logger.Info("  Common tools:")
		logger.Info("    - list_databases: List all available databases")
	}

	// If no database connections, register mock tools to ensure at least some tools are available
	if len(dbIDs) == 0 {
		logger.Info("No database connections available. Adding mock tools...")
		if err := toolRegistry.RegisterMockTools(ctx); err != nil {
			logger.Warn("Warning: error registering mock tools: %v", err)
		}
	}

	// Create a session store to track valid sessions
	sessions := make(map[string]bool)

	// Create a default session for easier testing
	defaultSessionID := "default-session"
	sessions[defaultSessionID] = true
	logger.Info("Created default session: %s", defaultSessionID)

	// Handle transport mode
	switch cfg.TransportMode {
	case "sse":
		logger.Info("Starting SSE server on port %d", cfg.ServerPort)

		// Configure base URL with explicit protocol
		baseURL := fmt.Sprintf("http://%s:%d", *serverHost, cfg.ServerPort)
		logger.Info("Using base URL: %s", baseURL)

		// Set logging mode based on configuration
		if cfg.DisableLogging {
			logger.Info("Logging in SSE transport is disabled")
			// Redirect standard output to null device if logging is disabled
			// This only works on Unix-like systems
			if err := os.Setenv("MCP_DISABLE_LOGGING", "true"); err != nil {
				logger.Warn("Warning: failed to set MCP_DISABLE_LOGGING env: %v", err)
			}
		}
		// Set the server address
		mcpServer.SetAddress(fmt.Sprintf(":%d", cfg.ServerPort))

		// Start the server
		errCh := make(chan error, 1)
		go func() {
			logger.Info("Starting server...")
			errCh <- mcpServer.ServeHTTP()
		}()

		// Wait for interrupt or error
		select {
		case err := <-errCh:
			logger.Error("Server error: %v", err)
			os.Exit(1)
		case <-stop:
			logger.Info("Shutting down server...")

			// Create shutdown context
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			// Shutdown the server
			if err := mcpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("Error during server shutdown: %v", err)
			}

			// Close database connections
			if err := dbtools.CloseDatabase(); err != nil {
				logger.Error("Error closing database connections: %v", err)
			}
		}

	case "stdio":
		// We can only log to stderr in stdio mode - NEVER stdout
		fmt.Fprintln(os.Stderr, "Starting STDIO server - all logging redirected to log files")

		// Create logs directory if not exists
		logsDir := "logs"
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			// Can't use logger.Warn as it might go to stdout
			fmt.Fprintf(os.Stderr, "Failed to create logs directory: %v\n", err)
		}

		// Export environment variables for the stdio server
		os.Setenv("MCP_DISABLE_LOGGING", "true")
		os.Setenv("DISABLE_LOGGING", "true")

		// Ensure standard logger doesn't output to stdout for any imported libraries
		// that use the standard log package
		logFile, err := os.OpenFile(filepath.Join(logsDir, "stdio-server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create stdio server log file: %v\n", err)
		} else {
			// Redirect all standard logging to the file
			log.SetOutput(logFile)
		}

		// Critical: Use ServeStdio WITHOUT any console output to stdout
		if err := mcpServer.ServeStdio(); err != nil {
			// Log error to stderr only - never stdout
			fmt.Fprintf(os.Stderr, "STDIO server error: %v\n", err)
			os.Exit(1)
		}

	default:
		logger.Error("Invalid transport mode: %s", cfg.TransportMode)
	}

	logger.Info("Server shutdown complete")
}
