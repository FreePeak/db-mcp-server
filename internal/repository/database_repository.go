// Package repository provides repository implementations for database operations.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// TODO: Implement caching layer for database metadata to improve performance
// TODO: Add observability with tracing and detailed metrics
// TODO: Improve concurrency handling with proper locking or atomic operations
// TODO: Consider using an interface-based approach for better testability
// TODO: Add comprehensive integration tests for different database types

// DatabaseRepository implements domain.DatabaseRepository
type DatabaseRepository struct{}

// NewDatabaseRepository creates a new database repository
func NewDatabaseRepository() *DatabaseRepository {
	return &DatabaseRepository{}
}

// GetDatabase retrieves a database by ID
func (r *DatabaseRepository) GetDatabase(id string) (domain.Database, error) {
	db, err := dbtools.GetDatabase(id)
	if err != nil {
		return nil, err
	}
	return &DatabaseAdapter{db: db}, nil
}

// ListDatabases returns a list of available database IDs
func (r *DatabaseRepository) ListDatabases() []string {
	return dbtools.ListDatabases()
}

// GetDatabaseType returns the type of a database by ID
func (r *DatabaseRepository) GetDatabaseType(id string) (string, error) {
	// Read database type from configuration without establishing a connection
	// The type is already validated when the connection is created, so we can trust it
	// This is especially important for lazy loading to avoid unnecessary connections during startup
	return dbtools.GetDatabaseType(id)
}

// IsLazyLoading returns whether lazy loading mode is enabled
func (r *DatabaseRepository) IsLazyLoading() bool {
	return dbtools.IsLazyLoading()
}

// DatabaseAdapter adapts the db.Database to the domain.Database interface
type DatabaseAdapter struct {
	db interface {
		Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
		Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
		BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
		IsReadOnly() bool
		Ping(ctx context.Context) error
		DB() *sql.DB
	}
}

// timeout applies the database's configured statement timeout so a runaway
// query cannot hold its connection indefinitely. Engines whose drivers
// support cancellation (PostgreSQL, MySQL) propagate it server-side; others
// are still bounded at this layer.
func (a *DatabaseAdapter) timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	var secs int
	if t, ok := a.db.(interface{ QueryTimeout() int }); ok {
		secs = t.QueryTimeout()
	}
	if secs <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(secs)*time.Second)
}

// Query executes a query on the database
func (a *DatabaseAdapter) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	tctx, cancel := a.timeout(ctx)
	rows, err := a.db.Query(tctx, query, args...)
	if err != nil {
		cancel()
		return nil, err
	}
	// The result set outlives this call, so cancellation travels with the
	// rows and fires when they are closed.
	return &RowsAdapter{rows: rows, cancel: cancel}, nil
}

// Exec executes a statement on the database
func (a *DatabaseAdapter) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	tctx, cancel := a.timeout(ctx)
	defer cancel()
	result, err := a.db.Exec(tctx, statement, args...)
	if err != nil {
		return nil, err
	}
	return &ResultAdapter{result: result}, nil
}

// Begin starts a new transaction
func (a *DatabaseAdapter) Begin(ctx context.Context, opts *domain.TxOptions) (domain.Tx, error) {
	txOpts := &sql.TxOptions{}
	if opts != nil {
		txOpts.ReadOnly = opts.ReadOnly
	}
	tctx, cancel := a.timeout(ctx)
	defer cancel() // only BeginTx consumes this context; later tx ops carry their own

	tx, err := a.db.BeginTx(tctx, txOpts)
	if err != nil {
		return nil, err
	}
	return &TxAdapter{tx: tx}, nil
}

// IsReadOnly reports whether the connection was opened in read-only mode.
// The use-case layer relies on this to enforce the per-database `read_only`
// configuration flag added for issue #41.
func (a *DatabaseAdapter) IsReadOnly() bool {
	return a.db.IsReadOnly()
}

// MaxRows returns the configured query row limit for this database.
func (a *DatabaseAdapter) MaxRows() int {
	if r, ok := a.db.(interface{ MaxRows() int }); ok {
		return r.MaxRows()
	}
	return 0
}

// QueryTimeout returns the configured statement timeout in seconds for this
// database, mirroring MaxRows so guardrails are observable above the
// repository layer.
func (a *DatabaseAdapter) QueryTimeout() int {
	if t, ok := a.db.(interface{ QueryTimeout() int }); ok {
		return t.QueryTimeout()
	}
	return 0
}

// Ping probes liveness of the underlying connection pool.
func (a *DatabaseAdapter) Ping(ctx context.Context) error {
	return a.db.Ping(ctx)
}

// HealthStats snapshots Go database/sql pool pressure for this database.
func (a *DatabaseAdapter) HealthStats() map[string]interface{} {
	s := a.db.DB().Stats()
	return map[string]interface{}{
		"pool_open_connections": s.OpenConnections,
		"pool_in_use":           s.InUse,
		"pool_idle":             s.Idle,
		"pool_wait_count":       s.WaitCount,
		"pool_wait_duration_ms": float64(s.WaitDuration.Microseconds()) / 1000.0,
	}
}

// RowsAdapter adapts sql.Rows to domain.Rows
type RowsAdapter struct {
	rows   *sql.Rows
	cancel context.CancelFunc // set when a statement timeout wraps the query context
}

// Close closes the rows
func (a *RowsAdapter) Close() error {
	err := a.rows.Close()
	if a.cancel != nil {
		a.cancel()
	}
	return err
}

// Columns returns the column names
func (a *RowsAdapter) Columns() ([]string, error) {
	return a.rows.Columns()
}

// Next advances to the next row
func (a *RowsAdapter) Next() bool {
	return a.rows.Next()
}

// Scan scans the current row
func (a *RowsAdapter) Scan(dest ...interface{}) error {
	return a.rows.Scan(dest...)
}

// Err returns any error that occurred during iteration
func (a *RowsAdapter) Err() error {
	return a.rows.Err()
}

// ResultAdapter adapts sql.Result to domain.Result
type ResultAdapter struct {
	result sql.Result
}

// RowsAffected returns the number of rows affected
func (a *ResultAdapter) RowsAffected() (int64, error) {
	return a.result.RowsAffected()
}

// LastInsertId returns the last insert ID
func (a *ResultAdapter) LastInsertId() (int64, error) {
	return a.result.LastInsertId()
}

// TxAdapter adapts sql.Tx to domain.Tx
type TxAdapter struct {
	tx *sql.Tx
}

// Commit commits the transaction
func (a *TxAdapter) Commit() error {
	return a.tx.Commit()
}

// Rollback rolls back the transaction
func (a *TxAdapter) Rollback() error {
	return a.tx.Rollback()
}

// Query executes a query within the transaction
func (a *TxAdapter) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	rows, err := a.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &RowsAdapter{rows: rows}, nil
}

// Exec executes a statement within the transaction
func (a *TxAdapter) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	result, err := a.tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	return &ResultAdapter{result: result}, nil
}
