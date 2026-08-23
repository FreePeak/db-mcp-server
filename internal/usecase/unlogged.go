package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Unlogged-table audit: CREATE UNLOGGED TABLE skips WAL — faster bulk
// loads, but the table is truncated on crash recovery and never
// replicated to standbys. A "temp" staging table that quietly became
// load-bearing is a data-loss incident waiting for the first crash.
// Only pg_class.relpersistence reveals them.

// unloggedTableQuery returns the SELECT for user-schema tables with
// relpersistence='u', or "" when unsupported.
func unloggedTableQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT n.nspname,
       c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relpersistence = 'u'
  AND c.relkind IN ('r', 'p')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname`
	default:
		return ""
	}
}

// ListUnloggedTables renders every user table that skips WAL; a clean
// result is stated explicitly.
func (uc *DatabaseUseCase) ListUnloggedTables(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := unloggedTableQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("persistence introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("persistence catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing unlogged-table rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var schema, table string
		if scanErr := rows.Scan(&schema, &table); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf(
			"- %s.%s: UNLOGGED — truncated on crash recovery and never replicated to standbys; convert with ALTER TABLE %s.%s SET LOGGED if it holds anything you cannot lose",
			schema, table, quoteIdent(schema), quoteIdent(table)))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate unlogged-table rows: %w", err)
	}

	if len(lines) == 0 {
		return "No unlogged tables — all user tables are crash-safe and replicated.", nil
	}
	return fmt.Sprintf("%d unlogged table(s):\n%s", len(lines), strings.Join(lines, "\n")), nil
}
