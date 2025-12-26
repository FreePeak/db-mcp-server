// Package dbtools provides helper functions and utilities for database operations
// in the DB MCP Server, including query building, schema exploration, and performance analysis.
package dbtools

import (
	"context"
	"database/sql"
)

// Database represents a database interface
// This is used in testing to provide a common interface
type Database interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Query executes a query and returns the result rows
func Query(ctx context.Context, db Database, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(ctx, query, args...)
}

// QueryRow executes a query and returns a single row
func QueryRow(ctx context.Context, db Database, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func Exec(ctx context.Context, db Database, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(ctx, query, args...)
}

// BeginTx starts a new transaction
func BeginTx(ctx context.Context, db Database, opts *sql.TxOptions) (*sql.Tx, error) {
	return db.BeginTx(ctx, opts)
}
