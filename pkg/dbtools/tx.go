package dbtools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/logger"
	"github.com/FreePeak/db-mcp-server/pkg/tools"
)

// Transaction state storage (in-memory)
var activeTransactions = make(map[string]*sql.Tx)

// createTransactionTool creates a tool for managing database transactions
func createTransactionTool() *tools.Tool {
	return &tools.Tool{
		Name:        "dbTransaction",
		Description: "Manage database transactions (begin, commit, rollback, execute within transaction)",
		Category:    "database",
		InputSchema: tools.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Action to perform (begin, commit, rollback, execute)",
					"enum":        []string{"begin", "commit", "rollback", "execute"},
				},
				"transactionId": map[string]interface{}{
					"type":        "string",
					"description": "Transaction ID (returned from begin, required for all other actions)",
				},
				"statement": map[string]interface{}{
					"type":        "string",
					"description": "SQL statement to execute (required for execute action)",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Parameters for the statement (for prepared statements)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"readOnly": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the transaction is read-only (for begin action)",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in milliseconds (default: 30000)",
				},
			},
			Required: []string{"action"},
		},
		Handler: handleTransaction,
	}
}

// handleTransaction handles the transaction tool execution
func handleTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Check if database is initialized
	if dbInstance == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Extract action
	action, ok := getStringParam(params, "action")
	if !ok {
		return nil, fmt.Errorf("action parameter is required")
	}

	// Handle different actions
	switch action {
	case "begin":
		return beginTransaction(ctx, params)
	case "commit":
		return commitTransaction(ctx, params)
	case "rollback":
		return rollbackTransaction(ctx, params)
	case "execute":
		return executeInTransaction(ctx, params)
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// beginTransaction starts a new transaction
func beginTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract timeout
	timeout := 30000 // Default timeout: 30 seconds
	if timeoutParam, ok := getIntParam(params, "timeout"); ok {
		timeout = timeoutParam
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Extract read-only flag
	readOnly := false
	if readOnlyParam, ok := params["readOnly"].(bool); ok {
		readOnly = readOnlyParam
	}

	// Set transaction options
	txOpts := &sql.TxOptions{
		ReadOnly: readOnly,
	}

	// Begin transaction
	tx, err := dbInstance.BeginTx(timeoutCtx, txOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Generate transaction ID
	txID := fmt.Sprintf("tx-%d", time.Now().UnixNano())

	// Store transaction
	activeTransactions[txID] = tx

	// Return transaction ID
	return map[string]interface{}{
		"transactionId": txID,
		"readOnly":      readOnly,
		"status":        "active",
	}, nil
}

// commitTransaction commits a transaction
func commitTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Get transaction
	tx, ok := activeTransactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Commit transaction
	err := tx.Commit()

	// Remove transaction from storage
	delete(activeTransactions, txID)

	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return success
	return map[string]interface{}{
		"transactionId": txID,
		"status":        "committed",
	}, nil
}

// rollbackTransaction rolls back a transaction
func rollbackTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Get transaction
	tx, ok := activeTransactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Rollback transaction
	err := tx.Rollback()

	// Remove transaction from storage
	delete(activeTransactions, txID)

	if err != nil {
		return nil, fmt.Errorf("failed to rollback transaction: %w", err)
	}

	// Return success
	return map[string]interface{}{
		"transactionId": txID,
		"status":        "rolled back",
	}, nil
}

// executeInTransaction executes a statement within a transaction
func executeInTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Get transaction
	tx, ok := activeTransactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Extract statement
	statement, ok := getStringParam(params, "statement")
	if !ok {
		return nil, fmt.Errorf("statement parameter is required")
	}

	// Extract statement parameters
	var statementParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		statementParams = make([]interface{}, len(paramsArray))
		copy(statementParams, paramsArray)
	}

	// Check if statement is a query or an execute statement
	isQuery := isQueryStatement(statement)

	// Get the performance analyzer
	analyzer := GetPerformanceAnalyzer()

	// Execute with performance tracking
	var finalResult interface{}
	var err error

	finalResult, err = analyzer.TrackQuery(ctx, statement, statementParams, func() (interface{}, error) {
		var result interface{}

		if isQuery {
			// Execute query within transaction
			rows, queryErr := tx.QueryContext(ctx, statement, statementParams...)
			if queryErr != nil {
				return nil, fmt.Errorf("failed to execute query in transaction: %w", queryErr)
			}
			defer func() {
				if closeErr := rows.Close(); closeErr != nil {
					logger.Error("Error closing rows: %v", closeErr)
				}
			}()

			// Convert rows to map
			results, convErr := rowsToMaps(rows)
			if convErr != nil {
				return nil, fmt.Errorf("failed to process query results in transaction: %w", convErr)
			}

			result = map[string]interface{}{
				"rows":  results,
				"count": len(results),
			}
		} else {
			// Execute statement within transaction
			execResult, execErr := tx.ExecContext(ctx, statement, statementParams...)
			if execErr != nil {
				return nil, fmt.Errorf("failed to execute statement in transaction: %w", execErr)
			}

			// Get affected rows
			rowsAffected, rowErr := execResult.RowsAffected()
			if rowErr != nil {
				rowsAffected = -1 // Unable to determine
			}

			// Get last insert ID (if applicable)
			lastInsertID, idErr := execResult.LastInsertId()
			if idErr != nil {
				lastInsertID = -1 // Unable to determine
			}

			result = map[string]interface{}{
				"rowsAffected": rowsAffected,
				"lastInsertId": lastInsertID,
			}
		}

		// Return results with transaction info
		return map[string]interface{}{
			"transactionId": txID,
			"statement":     statement,
			"params":        statementParams,
			"result":        result,
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return finalResult, nil
}

// isQueryStatement determines if a statement is a query (SELECT) or not
func isQueryStatement(statement string) bool {
	// Simple heuristic: if the statement starts with SELECT, it's a query
	// This is a simplification; a real implementation would use a proper SQL parser
	return len(statement) >= 6 && statement[0:6] == "SELECT"
}

// createMockTransactionTool creates a mock version of the transaction tool that works without database connection
func createMockTransactionTool() *tools.Tool {
	// Create the tool using the same schema as the real transaction tool
	tool := createTransactionTool()

	// Replace the handler with mock implementation
	tool.Handler = handleMockTransaction

	return tool
}

// Mock transaction state storage (in-memory)
var mockActiveTransactions = make(map[string]bool)

// handleMockTransaction is a mock implementation of the transaction handler
func handleMockTransaction(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	// Extract action parameter
	action, ok := getStringParam(params, "action")
	if !ok {
		return nil, fmt.Errorf("action parameter is required")
	}

	// Validate action
	validActions := map[string]bool{"begin": true, "commit": true, "rollback": true, "execute": true}
	if !validActions[action] {
		return nil, fmt.Errorf("invalid action: %s", action)
	}

	// Handle different actions
	switch action {
	case "begin":
		return handleMockBeginTransaction(params)
	case "commit":
		return handleMockCommitTransaction(params)
	case "rollback":
		return handleMockRollbackTransaction(params)
	case "execute":
		return handleMockExecuteTransaction(params)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// handleMockBeginTransaction handles the mock begin transaction action
func handleMockBeginTransaction(params map[string]interface{}) (interface{}, error) {
	// Extract read-only parameter (optional)
	readOnly, _ := params["readOnly"].(bool)

	// Generate a transaction ID
	txID := fmt.Sprintf("mock-tx-%d", time.Now().UnixNano())

	// Store in mock transaction state
	mockActiveTransactions[txID] = true

	// Return transaction info
	return map[string]interface{}{
		"transactionId": txID,
		"readOnly":      readOnly,
		"status":        "active",
	}, nil
}

// handleMockCommitTransaction handles the mock commit transaction action
func handleMockCommitTransaction(params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Verify transaction exists
	if !mockActiveTransactions[txID] {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Remove from active transactions
	delete(mockActiveTransactions, txID)

	// Return success
	return map[string]interface{}{
		"transactionId": txID,
		"status":        "committed",
	}, nil
}

// handleMockRollbackTransaction handles the mock rollback transaction action
func handleMockRollbackTransaction(params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Verify transaction exists
	if !mockActiveTransactions[txID] {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Remove from active transactions
	delete(mockActiveTransactions, txID)

	// Return success
	return map[string]interface{}{
		"transactionId": txID,
		"status":        "rolled back",
	}, nil
}

// handleMockExecuteTransaction handles the mock execute in transaction action
func handleMockExecuteTransaction(params map[string]interface{}) (interface{}, error) {
	// Extract transaction ID
	txID, ok := getStringParam(params, "transactionId")
	if !ok {
		return nil, fmt.Errorf("transactionId parameter is required")
	}

	// Verify transaction exists
	if !mockActiveTransactions[txID] {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	// Extract statement
	statement, ok := getStringParam(params, "statement")
	if !ok {
		return nil, fmt.Errorf("statement parameter is required")
	}

	// Extract statement parameters if provided
	var statementParams []interface{}
	if paramsArray, ok := getArrayParam(params, "params"); ok {
		statementParams = paramsArray
	}

	// Determine if this is a query or not (SELECT = query, otherwise execute)
	isQuery := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "SELECT")

	var result map[string]interface{}

	if isQuery {
		// Generate mock query results
		mockRows := []map[string]interface{}{
			{"column1": "mock value 1", "column2": 42},
			{"column1": "mock value 2", "column2": 84},
		}

		result = map[string]interface{}{
			"rows":  mockRows,
			"count": len(mockRows),
		}
	} else {
		// Generate mock execute results
		var rowsAffected int64 = 1
		var lastInsertID int64 = -1

		if strings.Contains(strings.ToUpper(statement), "INSERT") {
			lastInsertID = time.Now().Unix() % 1000
		} else if strings.Contains(strings.ToUpper(statement), "UPDATE") {
			rowsAffected = int64(1 + (time.Now().Unix() % 3))
		} else if strings.Contains(strings.ToUpper(statement), "DELETE") {
			rowsAffected = int64(time.Now().Unix() % 3)
		}

		result = map[string]interface{}{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertID,
		}
	}

	// Return results
	return map[string]interface{}{
		"transactionId": txID,
		"statement":     statement,
		"params":        statementParams,
		"result":        result,
	}, nil
}
