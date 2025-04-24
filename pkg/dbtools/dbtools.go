package dbtools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// RegisterMCPDatabaseTools registers database tools specifically formatted for MCP compatibility
func RegisterMCPDatabaseTools(registry *tools.Registry) error {
	// Get available databases
	dbs := ListDatabases()

	// If no databases are available, register mock tools
	if len(dbs) == 0 {
		return registerMCPMockTools(registry)
	}

	// Register MCP tools for each database
	for _, dbID := range dbs {
		// Register query tool for this database
		registry.RegisterTool(&tools.Tool{
			Name:        fmt.Sprintf("query_%s", dbID),
			Description: fmt.Sprintf("Execute SQL query on %s database", dbID),
			InputSchema: tools.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to execute",
					},
					"params": map[string]interface{}{
						"type":        "array",
						"description": "Query parameters",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				Required: []string{"query"},
			},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return handleQueryForDatabase(ctx, params, dbID)
			},
		})

		// Register execute tool for this database
		registry.RegisterTool(&tools.Tool{
			Name:        fmt.Sprintf("execute_%s", dbID),
			Description: fmt.Sprintf("Execute SQL statement on %s database", dbID),
			InputSchema: tools.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"statement": map[string]interface{}{
						"type":        "string",
						"description": "SQL statement to execute",
					},
					"params": map[string]interface{}{
						"type":        "array",
						"description": "Statement parameters",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				Required: []string{"statement"},
			},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return handleExecuteForDatabase(ctx, params, dbID)
			},
		})

		// Register transaction tool for this database
		registry.RegisterTool(&tools.Tool{
			Name:        fmt.Sprintf("transaction_%s", dbID),
			Description: fmt.Sprintf("Manage transactions on %s database", dbID),
			InputSchema: tools.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Transaction action (begin, commit, rollback, execute)",
						"enum":        []string{"begin", "commit", "rollback", "execute"},
					},
					"transactionId": map[string]interface{}{
						"type":        "string",
						"description": "Transaction ID (required for commit, rollback, execute)",
					},
					"statement": map[string]interface{}{
						"type":        "string",
						"description": "SQL statement to execute within transaction (required for execute)",
					},
					"params": map[string]interface{}{
						"type":        "array",
						"description": "Statement parameters",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"readOnly": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the transaction is read-only (for begin)",
					},
				},
				Required: []string{"action"},
			},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return handleTransactionForDatabase(ctx, params, dbID)
			},
		})

		// Register performance tool for this database
		registry.RegisterTool(&tools.Tool{
			Name:        fmt.Sprintf("performance_%s", dbID),
			Description: fmt.Sprintf("Analyze query performance on %s database", dbID),
			InputSchema: tools.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action (getSlowQueries, getMetrics, analyzeQuery, reset, setThreshold)",
						"enum":        []string{"getSlowQueries", "getMetrics", "analyzeQuery", "reset", "setThreshold"},
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "SQL query to analyze (required for analyzeQuery)",
					},
					"threshold": map[string]interface{}{
						"type":        "number",
						"description": "Slow query threshold in milliseconds (required for setThreshold)",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of results to return",
					},
				},
				Required: []string{"action"},
			},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return handlePerformanceForDatabase(ctx, params, dbID)
			},
		})

		// Register schema tool for this database
		registry.RegisterTool(&tools.Tool{
			Name:        fmt.Sprintf("schema_%s", dbID),
			Description: fmt.Sprintf("Get schema of on %s database", dbID),
			InputSchema: tools.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"random_string": map[string]interface{}{
						"type":        "string",
						"description": "Dummy parameter (optional)",
					},
				},
			},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return handleSchemaForDatabase(ctx, params, dbID)
			},
		})
	}

	// Register list_databases tool
	registry.RegisterTool(&tools.Tool{
		Name:        "list_databases",
		Description: "List all available databases on  database",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"random_string": map[string]interface{}{
					"type":        "string",
					"description": "Dummy parameter (optional)",
				},
			},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			dbs := ListDatabases()
			output := "Available databases:\n\n"
			for i, db := range dbs {
				output += fmt.Sprintf("%d. %s\n", i+1, db)
			}
			if len(dbs) == 0 {
				output += "No databases configured.\n"
			}
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": output},
				},
			}, nil
		},
	})

	return nil
}

// Helper function to create mock tools for MCP compatibility
func registerMCPMockTools(registry *tools.Registry) error {
	mockDBID := "mock"

	// Register mock query tool
	registry.RegisterTool(&tools.Tool{
		Name:        fmt.Sprintf("query_%s", mockDBID),
		Description: fmt.Sprintf("Execute SQL query on %s database", mockDBID),
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SQL query to execute",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Query parameters",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			Required: []string{"query"},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			query, _ := getStringParam(params, "query")
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Mock query executed:\n%s\n\nThis is a mock response.", query),
					},
				},
				"mock": true,
			}, nil
		},
	})

	// Register list_databases tool
	registry.RegisterTool(&tools.Tool{
		Name:        "list_databases",
		Description: "List all available databases on database",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"random_string": map[string]interface{}{
					"type":        "string",
					"description": "Dummy parameter (optional)",
				},
			},
		},
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Available databases:\n\n1. mock (not connected)\n",
					},
				},
				"mock": true,
			}, nil
		},
	})

	return nil
}

// Handler functions for specific databases
func handleQueryForDatabase(ctx context.Context, params map[string]interface{}, dbID string) (interface{}, error) {
	query, _ := getStringParam(params, "query")
	paramList, hasParams := getArrayParam(params, "params")

	var queryParams []interface{}
	if hasParams {
		queryParams = paramList
	}

	result, err := executeQueryWithParams(ctx, dbID, query, queryParams)
	if err != nil {
		return createErrorResponse(fmt.Sprintf("Error executing query on %s: %v", dbID, err)), nil
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("Results:\n\n%s", result)},
		},
	}, nil
}

func handleExecuteForDatabase(ctx context.Context, params map[string]interface{}, dbID string) (interface{}, error) {
	statement, _ := getStringParam(params, "statement")
	paramList, hasParams := getArrayParam(params, "params")

	var stmtParams []interface{}
	if hasParams {
		stmtParams = paramList
	}

	result, err := executeStatementWithParams(ctx, dbID, statement, stmtParams)
	if err != nil {
		return createErrorResponse(fmt.Sprintf("Error executing statement on %s: %v", dbID, err)), nil
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": result},
		},
	}, nil
}

func handleTransactionForDatabase(ctx context.Context, params map[string]interface{}, dbID string) (interface{}, error) {
	action, _ := getStringParam(params, "action")
	txID, hasTxID := getStringParam(params, "transactionId")
	statement, hasStatement := getStringParam(params, "statement")

	// Fix: properly handle type assertion
	readOnly := false
	if val, ok := params["readOnly"].(bool); ok {
		readOnly = val
	}

	paramList, hasParams := getArrayParam(params, "params")
	var stmtParams []interface{}
	if hasParams {
		stmtParams = paramList
	}

	switch action {
	case "begin":
		// Generate transaction ID if not provided
		if !hasTxID {
			txID = fmt.Sprintf("tx_%s_%d", dbID, time.Now().Unix())
		}

		// Start transaction
		db, err := GetDatabase(dbID)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to get database %s: %v", dbID, err)), nil
		}

		// Set read-only option if specified
		var opts *sql.TxOptions
		if readOnly {
			opts = &sql.TxOptions{ReadOnly: true}
		}

		tx, err := db.BeginTx(ctx, opts)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to begin transaction: %v", err)), nil
		}

		// Store transaction
		if err := storeTransaction(txID, tx); err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to store transaction: %v", err)), nil
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Transaction started"},
			},
			"metadata": map[string]interface{}{
				"transactionId": txID,
			},
		}, nil

	case "commit":
		if !hasTxID {
			return createErrorResponse("transactionId is required for commit action"), nil
		}

		tx, err := getTransaction(txID)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to get transaction %s: %v", txID, err)), nil
		}

		if err := tx.Commit(); err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to commit transaction: %v", err)), nil
		}

		// Remove transaction
		removeTransaction(txID)

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Transaction committed"},
			},
		}, nil

	case "rollback":
		if !hasTxID {
			return createErrorResponse("transactionId is required for rollback action"), nil
		}

		tx, err := getTransaction(txID)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to get transaction %s: %v", txID, err)), nil
		}

		if err := tx.Rollback(); err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to rollback transaction: %v", err)), nil
		}

		// Remove transaction
		removeTransaction(txID)

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Transaction rolled back"},
			},
		}, nil

	case "execute":
		if !hasTxID {
			return createErrorResponse("transactionId is required for execute action"), nil
		}

		if !hasStatement {
			return createErrorResponse("statement is required for execute action"), nil
		}

		tx, err := getTransaction(txID)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to get transaction %s: %v", txID, err)), nil
		}

		// Execute statement
		_, err = tx.Exec(statement, stmtParamsToInterfaceSlice(stmtParams)...)
		if err != nil {
			return createErrorResponse(fmt.Sprintf("Failed to execute statement in transaction: %v", err)), nil
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Statement executed in transaction"},
			},
		}, nil

	default:
		return createErrorResponse(fmt.Sprintf("Unknown transaction action: %s", action)), nil
	}
}

func handlePerformanceForDatabase(ctx context.Context, params map[string]interface{}, dbID string) (interface{}, error) {
	action, _ := getStringParam(params, "action")

	// Create response with basic info about the action
	limitVal, hasLimit := params["limit"].(float64)
	limit := 10
	if hasLimit {
		limit = int(limitVal)
	}

	// Return a basic mock response for the performance action
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Performance analysis for action '%s' on database '%s'\nLimit: %d\n", action, dbID, limit),
			},
		},
	}, nil
}

func handleSchemaForDatabase(ctx context.Context, params map[string]interface{}, dbID string) (interface{}, error) {
	// Try to get database schema
	db, err := GetDatabase(dbID)
	if err != nil {
		return createErrorResponse(fmt.Sprintf("Failed to get database %s: %v", dbID, err)), nil
	}

	// Get database type for more accurate schema reporting
	var dbType string
	switch db.DriverName() {
	case "mysql":
		dbType = "mysql"
	case "postgres":
		dbType = "postgres"
	default:
		dbType = "unknown"
	}

	// Get schema information
	schema := getBasicSchemaInfo(ctx, db, dbID, dbType)

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": fmt.Sprintf("Database Schema for %s:\n\n%v", dbID, schema),
			},
		},
	}, nil
}

// Helper function to get basic schema info
func getBasicSchemaInfo(ctx context.Context, db db.Database, dbID, dbType string) map[string]interface{} {
	result := map[string]interface{}{
		"database": dbID,
		"dbType":   dbType,
		"tables":   []map[string]string{},
	}

	// Try to get table list - a simple query that should work on most databases
	var query string
	switch dbType {
	case "mysql":
		query = "SHOW TABLES"
	case "postgres":
		query = "SELECT tablename AS TABLE_NAME FROM pg_catalog.pg_tables WHERE schemaname NOT IN ('pg_catalog', 'information_schema')"
	default:
		// Generic query that might work
		query = "SELECT name FROM sqlite_master WHERE type='table'"
	}

	rows, err := db.Query(ctx, query)
	if err != nil {
		// Return empty schema if query fails
		return result
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Warn("Error closing rows: %v", cerr)
		}
	}()

	tables := []map[string]string{}
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, map[string]string{"TABLE_NAME": tableName})
	}

	result["tables"] = tables
	return result
}

// Helper functions for parameter conversion
func stmtParamsToInterfaceSlice(params []interface{}) []interface{} {
	result := make([]interface{}, len(params))
	copy(result, params)
	return result
}

func createErrorResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": fmt.Sprintf("Error: %s", message)},
		},
		"isError": true,
	}
}

// executeQueryWithParams executes a query with the given parameters
func executeQueryWithParams(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	db, err := GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database %s: %w", dbID, err)
	}

	rows, err := db.Query(ctx, query, params...)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Warn("Error closing rows: %v", cerr)
		}
	}()

	// Convert rows to string representation
	result, err := formatRows(rows)
	if err != nil {
		return "", fmt.Errorf("failed to format rows: %w", err)
	}

	return result, nil
}

// executeStatementWithParams executes a statement with the given parameters
func executeStatementWithParams(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	db, err := GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database %s: %w", dbID, err)
	}

	result, err := db.Exec(ctx, statement, params...)
	if err != nil {
		return "", fmt.Errorf("failed to execute statement: %w", err)
	}

	// Get affected rows
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}

	// Get last insert ID (might not be supported by all databases)
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		lastInsertID = 0
	}

	return fmt.Sprintf("Statement executed successfully.\nRows affected: %d\nLast insert ID: %d", rowsAffected, lastInsertID), nil
}

// storeTransaction stores a transaction with the given ID
func storeTransaction(id string, tx *sql.Tx) error {
	// Check if transaction already exists
	_, exists := GetTransaction(id)
	if exists {
		return fmt.Errorf("transaction with ID %s already exists", id)
	}

	StoreTransaction(id, tx)
	return nil
}

// getTransaction retrieves a transaction by ID
func getTransaction(id string) (*sql.Tx, error) {
	tx, exists := GetTransaction(id)
	if !exists {
		return nil, fmt.Errorf("transaction with ID %s not found", id)
	}

	return tx, nil
}

// removeTransaction removes a transaction from storage
func removeTransaction(id string) {
	RemoveTransaction(id)
}

// formatRows formats SQL rows as a string table
func formatRows(rows *sql.Rows) (string, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	// Prepare column value holders
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// Build header
	var sb strings.Builder
	for i, col := range columns {
		if i > 0 {
			sb.WriteString("\t")
		}
		sb.WriteString(col)
	}
	sb.WriteString("\n")

	// Add separator
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	// Process rows
	rowCount := 0
	for rows.Next() {
		rowCount++
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}

		// Format row values
		for i, val := range values {
			if i > 0 {
				sb.WriteString("\t")
			}
			sb.WriteString(formatValue(val))
		}
		sb.WriteString("\n")
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	// Add total row count
	sb.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))
	return sb.String(), nil
}

// formatValue converts a value to string representation
func formatValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
