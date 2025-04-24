package dbtools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/logger"
	"github.com/FreePeak/db-mcp-server/pkg/tools"
)

// createQueryTool creates a tool for executing database queries
//
//nolint:unused // Retained for future use
func createQueryTool() *tools.Tool {
	return &tools.Tool{
		Name:        "dbQuery",
		Description: "Execute a database query that returns results",
		Category:    "database",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SQL query to execute",
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
				"database": map[string]interface{}{
					"type":        "string",
					"description": "Database ID to use (optional if only one database is configured)",
				},
			},
			Required: []string{"query", "database"},
		},
		Handler: handleQuery,
	}
}

// handleQuery handles the query tool execution
func handleQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if database manager is initialized
	if dbManager == nil {
		return nil, fmt.Errorf("database manager not initialized")
	}

	// Extract parameters
	query, ok := getStringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
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

	// Extract query parameters
	var queryParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		queryParams = make([]interface{}, len(paramsArray))
		copy(queryParams, paramsArray)
	}

	// Get the performance analyzer
	analyzer := GetPerformanceAnalyzer()

	// Execute query with performance tracking
	var result interface{}

	result, err = analyzer.TrackQuery(timeoutCtx, query, queryParams, func() (interface{}, error) {
		// Execute query
		rows, innerErr := db.Query(timeoutCtx, query, queryParams...)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to execute query: %w", innerErr)
		}
		defer cleanupRows(rows)

		// Convert rows to maps
		results, innerErr := rowsToMaps(rows)
		if innerErr != nil {
			return nil, fmt.Errorf("failed to process query results: %w", innerErr)
		}

		return map[string]interface{}{
			"results":  results,
			"query":    query,
			"params":   queryParams,
			"rowCount": len(results),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// createMockQueryTool creates a mock version of the query tool
//
//nolint:unused // Retained for future use
func createMockQueryTool() *tools.Tool {
	// Create the tool using the same schema as the real query tool
	tool := createQueryTool()

	// Replace the handler with mock implementation
	tool.Handler = handleMockQuery

	return tool
}

// handleMockQuery is a mock implementation of the query handler
//
//nolint:unused // Retained for future use
func handleMockQuery(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, ok := getStringParam(params, "query")
	if !ok {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Extract query parameters if provided
	var queryParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		queryParams = paramsArray
	}

	// Create mock results
	mockResults := []map[string]interface{}{
		{
			"id":        1,
			"name":      "Mock Result 1",
			"timestamp": time.Now().Format(time.RFC3339),
		},
		{
			"id":        2,
			"name":      "Mock Result 2",
			"timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339),
		},
	}

	return map[string]interface{}{
		"results":  mockResults,
		"query":    query,
		"params":   queryParams,
		"rowCount": len(mockResults),
	}, nil
}

// containsIgnoreCase checks if a string contains a substring, ignoring case
//
//nolint:unused // Retained for future use
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// cleanupRows ensures rows are closed properly
func cleanupRows(rows *sql.Rows) {
	if rows != nil {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}
}
