package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/db"
	"github.com/FreePeak/db-mcp-server/pkg/logger"
	"github.com/FreePeak/db-mcp-server/pkg/tools"
)

// TODO: Refactor database connection management to support connection pooling
// TODO: Add support for connection retries and circuit breaking
// TODO: Implement comprehensive metrics collection for database operations
// TODO: Consider using a context-aware connection management system
// TODO: Add support for database migrations and versioning
// TODO: Improve error handling with custom error types

// DatabaseType represents a supported database type
type DatabaseType string

const (
	// MySQL database type
	MySQL DatabaseType = "mysql"
	// Postgres database type
	Postgres DatabaseType = "postgres"
)

// Config represents database configuration
type Config struct {
	ConfigFile  string
	Connections []ConnectionConfig
}

// ConnectionConfig represents a single database connection configuration
type ConnectionConfig struct {
	ID       string       `json:"id"`
	Type     DatabaseType `json:"type"`
	Host     string       `json:"host"`
	Port     int          `json:"port"`
	Name     string       `json:"name"`
	User     string       `json:"user"`
	Password string       `json:"password"`
}

// MultiDBConfig represents configuration for multiple database connections
type MultiDBConfig struct {
	Connections []ConnectionConfig `json:"connections"`
}

// Database connection manager (singleton)
var (
	dbManager *db.Manager
)

// DatabaseConnectionInfo represents detailed information about a database connection
type DatabaseConnectionInfo struct {
	ID      string       `json:"id"`
	Type    DatabaseType `json:"type"`
	Host    string       `json:"host"`
	Port    int          `json:"port"`
	Name    string       `json:"name"`
	Status  string       `json:"status"`
	Latency string       `json:"latency,omitempty"`
}

// InitDatabase initializes the database connections
func InitDatabase(cfg *Config) error {
	// Create database manager
	dbManager = db.NewDBManager()

	var multiDBConfig *MultiDBConfig

	// If config file is provided, load it
	if cfg != nil && cfg.ConfigFile != "" {
		// Read config file
		configData, err := os.ReadFile(cfg.ConfigFile)
		if err != nil {
			logger.Warn("Warning: failed to read config file %s: %v", cfg.ConfigFile, err)
			// Don't return error, try other methods
		} else {
			// Parse config
			multiDBConfig = &MultiDBConfig{}
			if err := json.Unmarshal(configData, multiDBConfig); err != nil {
				logger.Warn("Warning: failed to parse config file %s: %v", cfg.ConfigFile, err)
				// Don't return error, try other methods
			} else {
				logger.Info("Loaded database config from file: %s", cfg.ConfigFile)
				// Debug logging of connection details
				for i, conn := range multiDBConfig.Connections {
					logger.Info("Connection [%d]: ID=%s, Type=%s, Host=%s, Port=%d, Name=%s",
						i, conn.ID, conn.Type, conn.Host, conn.Port, conn.Name)
				}
			}
		}
	}

	// If config was not loaded from file, try direct connections config
	if multiDBConfig == nil || len(multiDBConfig.Connections) == 0 {
		if cfg != nil && len(cfg.Connections) > 0 {
			// Use connections from direct config
			multiDBConfig = &MultiDBConfig{
				Connections: cfg.Connections,
			}
			logger.Info("Using database connections from direct configuration")
		} else {
			// Try to load from environment variable
			dbConfigJSON := os.Getenv("DB_CONFIG")
			if dbConfigJSON != "" {
				multiDBConfig = &MultiDBConfig{}
				if err := json.Unmarshal([]byte(dbConfigJSON), multiDBConfig); err != nil {
					logger.Warn("Warning: failed to parse DB_CONFIG environment variable: %v", err)
					// Don't return error, try legacy method
				} else {
					logger.Info("Loaded database config from DB_CONFIG environment variable")
				}
			}
		}
	}

	// If no config loaded yet, try legacy single connection from environment
	if multiDBConfig == nil || len(multiDBConfig.Connections) == 0 {
		// Create a single connection from environment variables
		dbType := os.Getenv("DB_TYPE")
		if dbType == "" {
			dbType = "mysql" // Default type
		}

		dbHost := os.Getenv("DB_HOST")
		dbPortStr := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPassword := os.Getenv("DB_PASSWORD")
		dbName := os.Getenv("DB_NAME")

		// If we have basic connection details, create a config
		if dbHost != "" && dbUser != "" {
			dbPort, err := strconv.Atoi(dbPortStr)
			if err != nil || dbPort == 0 {
				dbPort = 3306 // Default MySQL port
			}

			multiDBConfig = &MultiDBConfig{
				Connections: []ConnectionConfig{
					{
						ID:       "default",
						Type:     DatabaseType(dbType),
						Host:     dbHost,
						Port:     dbPort,
						Name:     dbName,
						User:     dbUser,
						Password: dbPassword,
					},
				},
			}
			logger.Info("Created database config from environment variables")
		}
	}

	// If still no config, return error
	if multiDBConfig == nil || len(multiDBConfig.Connections) == 0 {
		return fmt.Errorf("no database configuration provided")
	}

	// Convert config to JSON for loading
	configJSON, err := json.Marshal(multiDBConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal database config: %w", err)
	}

	if err := dbManager.LoadConfig(configJSON); err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}

	// Connect to all databases
	if err := dbManager.Connect(); err != nil {
		return fmt.Errorf("failed to connect to databases: %w", err)
	}

	// Log connected databases
	dbs := dbManager.ListDatabases()
	logger.Info("Connected to %d databases: %v", len(dbs), dbs)

	return nil
}

// CloseDatabase closes all database connections
func CloseDatabase() error {
	if dbManager == nil {
		return nil
	}
	return dbManager.CloseAll()
}

// GetDatabase returns a database instance by ID
func GetDatabase(id string) (db.Database, error) {
	if dbManager == nil {
		return nil, fmt.Errorf("database manager not initialized")
	}
	return dbManager.GetDatabase(id)
}

// ListDatabases returns a list of available database connections
func ListDatabases() []string {
	if dbManager == nil {
		return nil
	}
	return dbManager.ListDatabases()
}

// showConnectedDatabases returns information about all connected databases
func showConnectedDatabases(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	if dbManager == nil {
		return nil, fmt.Errorf("database manager not initialized")
	}

	var connections []DatabaseConnectionInfo
	dbIDs := ListDatabases()

	for _, dbID := range dbIDs {
		database, err := GetDatabase(dbID)
		if err != nil {
			continue
		}

		// Get connection details
		connInfo := DatabaseConnectionInfo{
			ID: dbID,
		}

		// Check connection status and measure latency
		start := time.Now()
		err = database.Ping(ctx)
		latency := time.Since(start)

		if err != nil {
			connInfo.Status = "disconnected"
			connInfo.Latency = "n/a"
		} else {
			connInfo.Status = "connected"
			connInfo.Latency = latency.String()
		}

		connections = append(connections, connInfo)
	}

	return connections, nil
}

// RegisterDatabaseTools registers all database tools with the provided registry
func RegisterDatabaseTools(registry *tools.Registry) error {
	// Register schema explorer tool
	registry.RegisterTool(&tools.Tool{
		Name:        "dbSchema",
		Description: "Auto-discover database structure and relationships",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database name to explore (optional, leave empty for all databases)",
				},
				"component": map[string]interface{}{
					"type":        "string",
					"description": "Component to explore (tables, columns, indices, or all)",
					"enum":        []string{"tables", "columns", "indices", "all"},
				},
				"table": map[string]interface{}{
					"type":        "string",
					"description": "Specific table to explore (optional)",
				},
			},
		},
		Handler: handleSchemaExplorer,
	})

	// Register query tool
	registry.RegisterTool(&tools.Tool{
		Name:        "dbQuery",
		Description: "Execute SQL query and return results",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SQL query to execute",
				},
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database ID to query (optional if only one database is configured)",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Parameters for the query (for prepared statements)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Query timeout in milliseconds (default: 5000)",
				},
			},
			Required: []string{"query"},
		},
		Handler: handleQuery,
	})

	// Register execute tool
	registry.RegisterTool(&tools.Tool{
		Name:        "dbExecute",
		Description: "Execute a database statement that doesn't return results (INSERT, UPDATE, DELETE, etc.)",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"statement": map[string]interface{}{
					"type":        "string",
					"description": "SQL statement to execute",
				},
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database ID to query (optional if only one database is configured)",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Parameters for the statement (for prepared statements)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Statement timeout in milliseconds (default: 5000)",
				},
			},
			Required: []string{"statement"},
		},
		Handler: handleExecute,
	})

	// Register list databases tool
	registry.RegisterTool(&tools.Tool{
		Name:        "dbList",
		Description: "List all available database connections",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"showStatus": map[string]interface{}{
					"type":        "boolean",
					"description": "Show connection status and latency",
				},
			},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			// Show connection status?
			showStatus, ok := params["showStatus"].(bool)
			if ok && showStatus {
				return showConnectedDatabases(ctx, params)
			}

			// Just list database IDs
			return ListDatabases(), nil
		},
	})

	// Register query builder tool
	registry.RegisterTool(&tools.Tool{
		Name:        "dbQueryBuilder",
		Description: "Build SQL queries visually",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform (build, validate, format)",
					"enum":        []string{"build", "validate", "format"},
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SQL query to validate or format",
				},
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database ID to use for validation",
				},
				"components": map[string]interface{}{
					"type":        "object",
					"description": "Query components (for build action)",
				},
			},
			Required: []string{"action"},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			// Just a placeholder for now
			actionVal, ok := params["action"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid 'action' parameter")
			}
			return fmt.Sprintf("Query builder %s action not implemented yet", actionVal), nil
		},
	})

	// Register Cursor-compatible tool handlers
	// TODO: Implement or import this function
	// tools.RegisterCursorCompatibleToolHandlers(registry)

	return nil
}

// PingDatabase pings a database to check the connection
func PingDatabase(db *sql.DB) error {
	return db.Ping()
}

// rowsToMaps converts sql.Rows to a slice of maps
func rowsToMaps(rows *sql.Rows) ([]map[string]interface{}, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Make a slice for the values
	values := make([]interface{}, len(columns))

	// Create references for the values
	valueRefs := make([]interface{}, len(columns))
	for i := range columns {
		valueRefs[i] = &values[i]
	}

	// Create the slice to store results
	var results []map[string]interface{}

	// Fetch rows
	for rows.Next() {
		// Scan the result into the pointers
		err := rows.Scan(valueRefs...)
		if err != nil {
			return nil, err
		}

		// Create a map for this row
		result := make(map[string]interface{})
		for i, column := range columns {
			val := values[i]

			// Handle null values
			if val == nil {
				result[column] = nil
				continue
			}

			// Convert bytes to string for easier JSON serialization
			if b, ok := val.([]byte); ok {
				result[column] = string(b)
			} else {
				result[column] = val
			}
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// getStringParam safely extracts a string parameter from the params map
func getStringParam(params map[string]interface{}, key string) (string, bool) {
	if val, ok := params[key].(string); ok {
		return val, true
	}
	return "", false
}

// getIntParam safely extracts an int parameter from the params map
func getIntParam(params map[string]interface{}, key string) (int, bool) {
	switch v := params[key].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

// getArrayParam safely extracts an array parameter from the params map
func getArrayParam(params map[string]interface{}, key string) ([]interface{}, bool) {
	if val, ok := params[key].([]interface{}); ok {
		return val, true
	}
	return nil, false
}

// _loadConfigFromFile loads database configuration from a file (currently unused)
func _loadConfigFromFile(cfg *Config) (*db.MultiDBConfig, error) {
	if cfg.ConfigFile == "" {
		return nil, fmt.Errorf("no config file specified")
	}

	// If path is not absolute, make it absolute
	absPath := cfg.ConfigFile
	if !filepath.IsAbs(absPath) {
		var err error
		absPath, err = filepath.Abs(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}
	}

	// Read configuration file
	configData, err := os.ReadFile(absPath)
	if err != nil {
		logger.Warn("Warning: failed to read config file %s: %v", cfg.ConfigFile, err)
		return nil, err
	}

	// Parse JSON
	var dbConfig db.MultiDBConfig
	if err := json.Unmarshal(configData, &dbConfig); err != nil {
		logger.Warn("Warning: failed to parse config file %s: %v", cfg.ConfigFile, err)
		return nil, err
	}
	logger.Info("Loaded database config from file: %s", cfg.ConfigFile)
	// Debug logging of connection details
	for i, conn := range dbConfig.Connections {
		logger.Info("Connection [%d]: ID=%s, Type=%s, Host=%s, Port=%d, Name=%s",
			i, conn.ID, conn.Type, conn.Host, conn.Port, conn.Name)
	}

	return &dbConfig, nil
}

// _getEnv gets an environment variable or returns a default value (currently unused)
func _getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// _getIntEnv gets an environment variable as an integer or returns a default value (currently unused)
func _getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

// _loadConfigFromEnv loads database configuration from the environment (currently unused)
func _loadConfigFromEnv() (*db.MultiDBConfig, error) {
	// Check if DB_CONFIG environment variable is set
	dbConfigEnv := os.Getenv("DB_CONFIG")
	if dbConfigEnv != "" {
		var dbConfig db.MultiDBConfig
		if err := json.Unmarshal([]byte(dbConfigEnv), &dbConfig); err != nil {
			logger.Warn("Warning: failed to parse DB_CONFIG environment variable: %v", err)
			return nil, err
		}
		logger.Info("Loaded database config from DB_CONFIG environment variable")
		return &dbConfig, nil
	}

	// Create config from individual environment variables
	// Load database configuration from environment variables
	dbType := _getEnv("DB_TYPE", "mysql")
	dbHost := _getEnv("DB_HOST", "localhost")
	dbPort := _getIntEnv("DB_PORT", 3306)
	dbUser := _getEnv("DB_USER", "")
	dbPass := _getEnv("DB_PASSWORD", "")
	dbName := _getEnv("DB_NAME", "")

	// Create a default configuration with a single connection
	dbConfig := &db.MultiDBConfig{
		Connections: []db.DatabaseConnectionConfig{
			{
				ID:       "default",
				Type:     dbType,
				Host:     dbHost,
				Port:     dbPort,
				User:     dbUser,
				Password: dbPass,
				Name:     dbName,
			},
		},
	}

	logger.Info("Created database config from environment variables")
	return dbConfig, nil
}

// GetDatabaseQueryTimeout returns the query timeout for a database in milliseconds
func GetDatabaseQueryTimeout(db db.Database) int {
	// Get the query timeout from the database configuration
	// Default to 30 seconds (30000ms) if not configured
	defaultTimeout := 30000 // ms

	if dbConfig, ok := db.(interface{ QueryTimeout() int }); ok {
		if timeout := dbConfig.QueryTimeout(); timeout > 0 {
			return timeout * 1000 // Convert from seconds to milliseconds
		}
	}

	return defaultTimeout
}
