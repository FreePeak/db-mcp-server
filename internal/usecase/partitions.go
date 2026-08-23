package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Partition listing: a large table may be partitioned, and nothing on
// the tool surface says so — an agent writing queries against the
// parent cannot see child partitions, their bounds, or where rows live.
// One catalog read per engine makes the layout explicit.

// partitionQuery returns the engine's per-table partition SELECT
// bound to one parent table, or "" when unsupported.
func partitionQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT c.relname AS partition_name,
       '' AS bound,
       GREATEST(c.reltuples::bigint, 0) AS row_estimate,
       pg_size_pretty(pg_total_relation_size(c.oid)) AS total_size
FROM pg_inherits i
JOIN pg_class c ON c.oid = i.inhrelid
WHERE i.inhparent = $1::regclass
ORDER BY c.relname`
	case "mysql", "mariadb":
		return `SELECT PARTITION_NAME,
       COALESCE(PARTITION_DESCRIPTION, '') AS bound,
       COALESCE(TABLE_ROWS, 0) AS row_estimate,
       'n/a' AS total_size
FROM information_schema.PARTITIONS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
ORDER BY PARTITION_ORDINAL_POSITION`
	default:
		return ""
	}
}

// ListPartitions renders a table's child partitions with bounds, row
// estimates, and sizes; "not partitioned" is stated explicitly rather
// than implied by an empty list.
func (uc *DatabaseUseCase) ListPartitions(ctx context.Context, dbID, table string) (string, error) {
	if !isPlainIdentifier(table) {
		return "", fmt.Errorf("invalid table name %q", table)
	}
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := partitionQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("partition introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q, table)
	if err != nil {
		return "", fmt.Errorf("partition catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing partition rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name, bound, size string
		var rowEst int64
		if scanErr := rows.Scan(&name, &bound, &rowEst, &size); scanErr != nil {
			continue // unscannable row: skip rather than fail the listing
		}
		line := fmt.Sprintf("- %s", name)
		if bound != "" {
			line += fmt.Sprintf(" (bound: %s)", bound)
		}
		line += fmt.Sprintf(": ~%d row(s), %s", rowEst, size)
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate partition rows: %w", err)
	}

	if len(lines) == 0 {
		return fmt.Sprintf("%s is not partitioned.", table), nil
	}
	return fmt.Sprintf("%d partition(s) of %s:\n%s", len(lines), table, strings.Join(lines, "\n")), nil
}
