// Package usecase provides business logic for database operations and queries.
package usecase

import (
	"context"
	"fmt"
	"sync"
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

// OracleQueryFactory creates queries for Oracle
type OracleQueryFactory struct{}

// GetTablesQueries returns table queries for Oracle
func (f *OracleQueryFactory) GetTablesQueries() []string {
	return []string{
		// User's own tables (most common)
		"SELECT table_name FROM user_tables ORDER BY table_name",
		// All tables accessible to user
		"SELECT table_name FROM all_tables WHERE owner NOT IN ('SYS', 'SYSTEM', 'OUTLN', 'XDB', 'CTXSYS', 'MDSYS', 'WMSYS', 'ORDSYS', 'ORDDATA', 'APEX_030200', 'APEX_040000', 'APEX_040100', 'APEX_040200') ORDER BY table_name",
		// All tables with owner prefix
		"SELECT owner || '.' || table_name AS table_name FROM all_tables WHERE owner NOT IN ('SYS', 'SYSTEM') ORDER BY owner, table_name",
	}
}

// SQLiteQueryFactory creates queries for SQLite (including CozoDB with SQLite storage)
type SQLiteQueryFactory struct{}

// GetTablesQueries returns table queries for SQLite
func (f *SQLiteQueryFactory) GetTablesQueries() []string {
	return []string{
		// Primary: sqlite_master approach for user tables
		"SELECT name AS table_name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'",
		// Fallback: pragma_table_list approach
		"SELECT name AS table_name FROM pragma_table_list() WHERE type='table' AND schema='main' AND name NOT LIKE 'sqlite_%'",
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
	case "oracle":
		return &OracleQueryFactory{}
	case "sqlite", "sqlite3":
		return &SQLiteQueryFactory{}
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

	// transactions holds live transactions keyed by their generated
	// transaction ID so begin/execute/commit/rollback act on the same
	// underlying transaction instead of stubbed no-ops.
	txMu         sync.Mutex
	transactions map[string]domain.Tx
}

// NewDatabaseUseCase creates a new database use case
func NewDatabaseUseCase(repo domain.DatabaseRepository) *DatabaseUseCase {
	return &DatabaseUseCase{
		repo:         repo,
		transactions: make(map[string]domain.Tx),
	}
}

func (uc *DatabaseUseCase) storeTx(id string, tx domain.Tx) {
	uc.txMu.Lock()
	defer uc.txMu.Unlock()
	uc.transactions[id] = tx
}

func (uc *DatabaseUseCase) getTx(id string) (domain.Tx, bool) {
	uc.txMu.Lock()
	defer uc.txMu.Unlock()
	tx, ok := uc.transactions[id]
	return tx, ok
}

// takeTx removes and returns the transaction registered under id.
func (uc *DatabaseUseCase) takeTx(id string) (domain.Tx, bool) {
	uc.txMu.Lock()
	defer uc.txMu.Unlock()
	tx, ok := uc.transactions[id]
	if ok {
		delete(uc.transactions, id)
	}
	return tx, ok
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

	// Read-only enforcement: the query tool previously accepted any
	// statement, so an agent could mutate a read_only database through it
	// even though the execute tool refused writes. Classify the statement
	// first and fail closed before touching the database.
	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}

	// Execute query
	rows, err := db.Query(ctx, query, params...)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()

	return formatQueryResults(rows, db.MaxRows())
}

// ExecuteQueryMasked executes a query with optional PII masking applied to
// result cells. mask=false preserves the legacy raw behavior.
func (uc *DatabaseUseCase) ExecuteQueryMasked(ctx context.Context, dbID, query string, params []interface{}, mask bool) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}

	rows, err := db.Query(ctx, query, params...)
	if err != nil {
		return "", fmt.Errorf("query execution failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()

	return renderQueryResults(rows, db.MaxRows(), mask)
}

// formatQueryResults renders query results as text, stopping after maxRows
// rows when maxRows > 0 so large result sets cannot flood the client's
// context window. The caller owns closing rows.
func formatQueryResults(rows domain.Rows, maxRows int) (string, error) {
	return renderQueryResults(rows, maxRows, false)
}

// ExecuteStatement executes a SQL statement (INSERT, UPDATE, DELETE)
func (uc *DatabaseUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	// Read-only enforcement: refuse any data-modifying statement when the
	// connection was opened with read_only: true. This resolves issue #41
	// by giving operators a per-database guardrail for production access.
	if db.IsReadOnly() {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed", dbID)
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

// ExecuteTransaction executes operations in a transaction. Actions:
//   - "begin": opens a real transaction and returns its ID in metadata
//   - "execute": runs a statement inside the transaction identified by txID
//   - "commit": commits and retires the transaction ID
//   - "rollback": rolls back and retires the transaction ID
func (uc *DatabaseUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string,
	statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {

	switch action {
	case "begin":
		db, err := uc.repo.GetDatabase(dbID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get database: %w", err)
		}

		// Read-only enforcement: refuse write transactions when the
		// connection was opened with read_only: true (issue #41).
		if !readOnly && db.IsReadOnly() {
			return "", nil, fmt.Errorf("database %q is configured as read-only; write transactions are not allowed", dbID)
		}

		tx, err := db.Begin(ctx, &domain.TxOptions{ReadOnly: readOnly})
		if err != nil {
			return "", nil, fmt.Errorf("failed to start transaction: %w", err)
		}

		newTxID := fmt.Sprintf("tx_%s_%d", dbID, timeNowUnixNano())
		uc.storeTx(newTxID, tx)

		return "Transaction started", map[string]interface{}{"transactionId": newTxID}, nil

	case "commit":
		tx, ok := uc.takeTx(txID)
		if !ok {
			return "", nil, fmt.Errorf("unknown transaction ID %q; use the transactionId returned by a previous begin action", txID)
		}
		if err := tx.Commit(); err != nil {
			return "", nil, fmt.Errorf("failed to commit transaction %q: %w", txID, err)
		}
		return "Transaction committed", map[string]interface{}{"transactionId": txID}, nil

	case "rollback":
		tx, ok := uc.takeTx(txID)
		if !ok {
			return "", nil, fmt.Errorf("unknown transaction ID %q; use the transactionId returned by a previous begin action", txID)
		}
		if err := tx.Rollback(); err != nil {
			return "", nil, fmt.Errorf("failed to roll back transaction %q: %w", txID, err)
		}
		return "Transaction rolled back", map[string]interface{}{"transactionId": txID}, nil

	case "execute":
		db, err := uc.repo.GetDatabase(dbID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get database: %w", err)
		}
		tx, ok := uc.getTx(txID)
		if !ok {
			return "", nil, fmt.Errorf("unknown transaction ID %q; use the transactionId returned by a previous begin action", txID)
		}

		// Defense-in-depth read-only guard for engines that do not enforce
		// read-only transactions themselves.
		if db.IsReadOnly() && IsWriteStatement(statement) {
			return "", nil, fmt.Errorf("database %q is configured as read-only; write statements are not allowed inside transactions", dbID)
		}

		result, err := tx.Exec(ctx, statement, params...)
		if err != nil {
			return "", nil, fmt.Errorf("statement execution failed in transaction: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			rowsAffected = 0
		}
		lastInsertID, err := result.LastInsertId()
		if err != nil {
			lastInsertID = 0
		}

		return fmt.Sprintf("Statement executed in transaction.\nRows affected: %d\nLast insert ID: %d", rowsAffected, lastInsertID),
			map[string]interface{}{"transactionId": txID}, nil

	default:
		return "", nil, fmt.Errorf("invalid transaction action: %s", action)
	}
}

// Helper function to get a nanosecond-resolution timestamp for unique
// transaction IDs under rapid successive calls.
func timeNowUnixNano() int64 {
	return time.Now().UnixNano()
}

// GetDatabaseType returns the type of a database by ID
func (uc *DatabaseUseCase) GetDatabaseType(dbID string) (string, error) {
	return uc.repo.GetDatabaseType(dbID)
}

// IsLazyLoading returns whether lazy loading mode is enabled
func (uc *DatabaseUseCase) IsLazyLoading() bool {
	return uc.repo.IsLazyLoading()
}
