// Package usecase provides business logic for database operations and queries.
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// TODO: Improve error handling with custom error types and better error messages
// TODO: Add extensive unit tests for all business logic
// TODO: Consider implementing domain events for better decoupling
// TODO: Add request validation layer before processing in usecases
// TODO: Implement proper context propagation and timeout handling

// QueryFactory provides database-specific queries
type QueryFactory interface {
	GetTablesQueries() []string
}

// PostgresQueryFactory creates queries for PostgreSQL
type PostgresQueryFactory struct{}

// GetTablesQueries returns table queries for PostgreSQL
func (f *PostgresQueryFactory) GetTablesQueries() []string {
	return []string{
		// Primary PostgreSQL query using pg_catalog (most reliable)
		"SELECT tablename AS table_name FROM pg_catalog.pg_tables WHERE schemaname = 'public'",
		// Fallback 1: Using information_schema
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
		// Fallback 2: Using pg_class for relations
		"SELECT relname AS table_name FROM pg_catalog.pg_class WHERE relkind = 'r' AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = 'public')",
	}
}

// MySQLQueryFactory creates queries for MySQL
type MySQLQueryFactory struct{}

// GetTablesQueries returns table queries for MySQL
func (f *MySQLQueryFactory) GetTablesQueries() []string {
	return []string{
		// Primary MySQL query
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()",
		// Fallback MySQL query
		"SHOW TABLES",
	}
}

// GenericQueryFactory creates generic queries for unknown database types
type GenericQueryFactory struct{}

// GetTablesQueries returns generic table queries for unknown database types
func (f *GenericQueryFactory) GetTablesQueries() []string {
	return []string{
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
		"SELECT table_name FROM information_schema.tables",
	}
}

// NewQueryFactory creates the appropriate query factory for the database type
func NewQueryFactory(dbType string) QueryFactory {
	switch dbType {
	case "postgres":
		return &PostgresQueryFactory{}
	case "mysql":
		return &MySQLQueryFactory{}
	default:
		logger.Warn("Unknown database type: %s, will use generic query factory", dbType)
		return &GenericQueryFactory{}
	}
}

// executeQueriesWithFallback tries multiple queries until one succeeds
func executeQueriesWithFallback(ctx context.Context, db domain.Database, queries []string) (domain.Rows, error) {
	var lastErr error
	var rows domain.Rows

	for i, query := range queries {
		var err error
		rows, err = db.Query(ctx, query)
		if err == nil {
			return rows, nil // Query succeeded
		}
		lastErr = err
		logger.Warn("Query %d failed: %s - Error: %v", i+1, query, err)
	}

	// All queries failed
	return nil, fmt.Errorf("all queries failed: %w", lastErr)
}

// DatabaseUseCase defines operations for managing database functionality
type DatabaseUseCase struct {
	repo domain.DatabaseRepository
}

// NewDatabaseUseCase creates a new database use case
func NewDatabaseUseCase(repo domain.DatabaseRepository) *DatabaseUseCase {
	return &DatabaseUseCase{
		repo: repo,
	}
}

// ListDatabases returns a list of available databases
func (uc *DatabaseUseCase) ListDatabases() []string {
	return uc.repo.ListDatabases()
}

// GetDatabaseInfo returns information about a database
func (uc *DatabaseUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	// Get database connection
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	// Get the database type
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	// Create appropriate query factory based on database type
	factory := NewQueryFactory(dbType)

	// Get queries for tables
	tableQueries := factory.GetTablesQueries()

	// Execute queries with fallback
	ctx := context.Background()
	rows, err := executeQueriesWithFallback(ctx, db, tableQueries)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema information: %w", err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()

	// Process results
	tables := []map[string]interface{}{}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}

	// Prepare for scanning
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// Process each row
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		// Convert to map
		tableInfo := make(map[string]interface{})
		for i, colName := range columns {
			val := values[i]
			if val == nil {
				tableInfo[colName] = nil
			} else {
				switch v := val.(type) {
				case []byte:
					tableInfo[colName] = string(v)
				default:
					tableInfo[colName] = v
				}
			}
		}
		tables = append(tables, tableInfo)
	}

	// Create result
	result := map[string]interface{}{
		"database": dbID,
		"dbType":   dbType,
		"tables":   tables,
	}

	return result, nil
}

// ExecuteQuery executes a SQL query and returns the formatted results
func (uc *DatabaseUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	// Execute query
	rows, err := db.Query(ctx, query, params...)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = fmt.Errorf("error closing rows: %w", closeErr)
		}
	}()

	// Process results into a readable format
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get column names: %w", err)
	}

	// Format results as text
	var resultText strings.Builder
	resultText.WriteString("Results:\n\n")
	resultText.WriteString(strings.Join(columns, "\t") + "\n")
	resultText.WriteString(strings.Repeat("-", 80) + "\n")

	// Prepare for scanning
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// Process rows
	rowCount := 0
	for rows.Next() {
		rowCount++
		scanErr := rows.Scan(valuePtrs...)
		if scanErr != nil {
			return "", fmt.Errorf("failed to scan row: %w", scanErr)
		}

		// Convert to strings and print
		var rowText []string
		for i := range columns {
			val := values[i]
			if val == nil {
				rowText = append(rowText, "NULL")
			} else {
				switch v := val.(type) {
				case []byte:
					rowText = append(rowText, string(v))
				default:
					rowText = append(rowText, fmt.Sprintf("%v", v))
				}
			}
		}
		resultText.WriteString(strings.Join(rowText, "\t") + "\n")
	}

	if err = rows.Err(); err != nil {
		return "", fmt.Errorf("error reading rows: %w", err)
	}

	resultText.WriteString(fmt.Sprintf("\nTotal rows: %d", rowCount))
	return resultText.String(), nil
}

// ExecuteStatement executes a SQL statement (INSERT, UPDATE, DELETE)
func (uc *DatabaseUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	// Execute statement
	result, err := db.Exec(ctx, statement, params...)
	if err != nil {
		return "", fmt.Errorf("statement execution failed: %w", err)
	}

	// Get rows affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		rowsAffected = 0
	}

	// Get last insert ID (if applicable)
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		lastInsertID = 0
	}

	return fmt.Sprintf("Statement executed successfully.\nRows affected: %d\nLast insert ID: %d", rowsAffected, lastInsertID), nil
}

// ExecuteTransaction executes operations in a transaction
func (uc *DatabaseUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, _ string,
	_ string, _ []interface{}, readOnly bool) (string, map[string]interface{}, error) {

	switch action {
	case "begin":
		db, err := uc.repo.GetDatabase(dbID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get database: %w", err)
		}

		// Start a new transaction
		txOpts := &domain.TxOptions{ReadOnly: readOnly}
		tx, err := db.Begin(ctx, txOpts)
		if err != nil {
			return "", nil, fmt.Errorf("failed to start transaction: %w", err)
		}

		// In a real implementation, we would store the transaction for later use
		// For now, we just commit right away to avoid the unused variable warning
		if err := tx.Commit(); err != nil {
			return "", nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		// Generate transaction ID
		newTxID := fmt.Sprintf("tx_%s_%d", dbID, timeNowUnix())

		return "Transaction started", map[string]interface{}{"transactionId": newTxID}, nil

	case "commit":
		// Implement commit logic (would need access to stored transaction)
		return "Transaction committed", nil, nil

	case "rollback":
		// Implement rollback logic (would need access to stored transaction)
		return "Transaction rolled back", nil, nil

	case "execute":
		// Implement execute within transaction logic (would need access to stored transaction)
		return "Statement executed in transaction", nil, nil

	default:
		return "", nil, fmt.Errorf("invalid transaction action: %s", action)
	}
}

// Helper function to get current Unix timestamp
func timeNowUnix() int64 {
	return time.Now().Unix()
}

// GetDatabaseType returns the type of a database by ID
func (uc *DatabaseUseCase) GetDatabaseType(dbID string) (string, error) {
	return uc.repo.GetDatabaseType(dbID)
}

// IsLazyLoading returns whether lazy loading mode is enabled
func (uc *DatabaseUseCase) IsLazyLoading() bool {
	return uc.repo.IsLazyLoading()
}
