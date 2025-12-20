// Package domain defines core domain interfaces and types for database operations.
package domain

import (
	"context"
)

// Database represents a database connection and operations
type Database interface {
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	Exec(ctx context.Context, statement string, args ...interface{}) (Result, error)
	Begin(ctx context.Context, opts *TxOptions) (Tx, error)
}

// Rows represents database query results
type Rows interface {
	Close() error
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

// Result represents the result of a database operation
type Result interface {
	RowsAffected() (int64, error)
	LastInsertId() (int64, error)
}

// Tx represents a database transaction
type Tx interface {
	Commit() error
	Rollback() error
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	Exec(ctx context.Context, statement string, args ...interface{}) (Result, error)
}

// TxOptions represents options for starting a transaction
type TxOptions struct {
	ReadOnly bool
}

// PerformanceAnalyzer for analyzing database query performance
type PerformanceAnalyzer interface {
	GetSlowQueries(limit int) ([]SlowQuery, error)
	GetMetrics() (PerformanceMetrics, error)
	AnalyzeQuery(query string) (QueryAnalysis, error)
	Reset() error
	SetThreshold(threshold int) error
}

// SlowQuery represents a slow query that has been recorded
type SlowQuery struct {
	Query     string
	Duration  float64
	Timestamp string
}

// PerformanceMetrics represents database performance metrics
type PerformanceMetrics struct {
	TotalQueries  int
	AvgDuration   float64
	MaxDuration   float64
	SlowQueries   int
	Threshold     int
	LastResetTime string
}

// QueryAnalysis represents the analysis of a SQL query
type QueryAnalysis struct {
	Query       string
	ExplainPlan string
}

// SchemaInfo represents database schema information
type SchemaInfo interface {
	GetTables() ([]string, error)
	GetColumns(table string) ([]ColumnInfo, error)
	GetIndexes(table string) ([]IndexInfo, error)
	GetConstraints(table string) ([]ConstraintInfo, error)
}

// ColumnInfo represents information about a database column
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Default  string
}

// IndexInfo represents information about a database index
type IndexInfo struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
	Primary bool
}

// ConstraintInfo represents information about a database constraint
type ConstraintInfo struct {
	Name              string
	Type              string
	Table             string
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}

// DatabaseRepository defines methods for managing database connections
type DatabaseRepository interface {
	GetDatabase(id string) (Database, error)
	ListDatabases() []string
	GetDatabaseType(id string) (string, error)
	IsLazyLoading() bool
}
