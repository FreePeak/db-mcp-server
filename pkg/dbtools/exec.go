package dbtools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/tools"
)

// createExecuteTool creates a tool for executing database statements that don't return rows
//
//nolint:unused // Retained for future use
func createExecuteTool() *tools.Tool {
	return &tools.Tool{
		Name:        "dbExecute",
		Description: "Execute a database statement that doesn't return results (INSERT, UPDATE, DELETE, etc.)",
		Category:    "database",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"statement": map[string]interface{}{
					"type":        "string",
					"description": "SQL statement to execute",
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
					"description": "Execution timeout in milliseconds (default: 5000)",
				},
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database ID to use (optional if only one database is configured)",
				},
			},
			Required: []string{"statement", "database"},
		},
		Handler: handleExecute,
	}
}

// handleExecute handles the execute tool execution
func handleExecute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if database manager is initialized
	if dbManager == nil {
		return nil, fmt.Errorf("database manager not initialized")
	}

	// Extract parameters
	statement, ok := getStringParam(params, "statement")
	if !ok {
		return nil, fmt.Errorf("statement parameter is required")
	}

	// Get database ID
	databaseID, ok := getStringParam(params, "database")
	if !ok {
		return nil, fmt.Errorf("database parameter is required")
	}

	// Get database instance
	db, err := dbManager.GetDatabase(databaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Extract timeout
	dbTimeout := db.QueryTimeout() * 1000 // Convert from seconds to milliseconds
	timeout := dbTimeout                  // Default to the database's query timeout
	if timeoutParam, ok := getIntParam(params, "timeout"); ok {
		timeout = timeoutParam
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Extract statement parameters
	var statementParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		statementParams = make([]interface{}, len(paramsArray))
		copy(statementParams, paramsArray)
	}

	// Get the performance analyzer
	analyzer := GetPerformanceAnalyzer()

	// Execute statement with performance tracking
	var result interface{}

	result, err = analyzer.TrackQuery(timeoutCtx, statement, statementParams, func() (interface{}, error) {
		// Execute statement
		sqlResult, innerErr := db.Exec(timeoutCtx, statement, statementParams...)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to execute statement: %w", innerErr)
		}

		// Get affected rows
		rowsAffected, rowsErr := sqlResult.RowsAffected()
		if rowsErr != nil {
			rowsAffected = -1 // Unable to determine
		}

		// Get last insert ID (if applicable)
		lastInsertID, idErr := sqlResult.LastInsertId()
		if idErr != nil {
			lastInsertID = -1 // Unable to determine
		}

		// Return results
		return map[string]interface{}{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertID,
			"statement":    statement,
			"params":       statementParams,
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// createMockExecuteTool creates a mock version of the execute tool that works without database connection
//
//nolint:unused // Retained for future use
func createMockExecuteTool() *tools.Tool {
	// Create the tool using the same schema as the real execute tool
	tool := createExecuteTool()

	// Replace the handler with mock implementation
	tool.Handler = handleMockExecute

	return tool
}

// handleMockExecute is a mock implementation of the execute handler
//
//nolint:unused // Retained for future use
func handleMockExecute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	statement, ok := getStringParam(params, "statement")
	if !ok {
		return nil, fmt.Errorf("statement parameter is required")
	}

	// Extract statement parameters if provided
	var statementParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		statementParams = paramsArray
	}

	// Simulate results based on statement
	var rowsAffected int64 = 1
	var lastInsertID int64 = -1

	// Simple pattern matching to provide realistic mock results
	if strings.Contains(strings.ToUpper(statement), "INSERT") {
		// For INSERT statements, generate a mock last insert ID
		lastInsertID = time.Now().Unix() % 1000 // Generate a pseudo-random ID based on current time
		rowsAffected = 1
	} else if strings.Contains(strings.ToUpper(statement), "UPDATE") {
		// For UPDATE statements, simulate affecting 1-3 rows
		rowsAffected = int64(1 + (time.Now().Unix() % 3))
	} else if strings.Contains(strings.ToUpper(statement), "DELETE") {
		// For DELETE statements, simulate affecting 0-2 rows
		rowsAffected = int64(time.Now().Unix() % 3)
	}

	// Return results in the same format as the real execute tool
	return map[string]interface{}{
		"rowsAffected": rowsAffected,
		"lastInsertId": lastInsertID,
		"statement":    statement,
		"params":       statementParams,
	}, nil
}
