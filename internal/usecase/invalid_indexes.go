package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Invalid-index audit: a crashed CREATE INDEX CONCURRENTLY leaves an
// INVALID index behind — the planner ignores it (so it helps no read)
// but every write still maintains it (so it costs like an index).
// Nothing else on the tool surface distinguishes valid from invalid.

// invalidIndexQuery returns the SELECT for user-schema indexes with
// indisvalid=false, or "" when unsupported.
func invalidIndexQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT n.nspname,
       t.relname AS table_name,
       i.relname AS index_name
FROM pg_index x
JOIN pg_class i ON i.oid = x.indexrelid
JOIN pg_class t ON t.oid = x.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE NOT x.indisvalid
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, t.relname, i.relname`
	default:
		return ""
	}
}

// ListInvalidIndexes renders every invalid user index; a clean result
// is stated explicitly.
func (uc *DatabaseUseCase) ListInvalidIndexes(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := invalidIndexQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("index-validity introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("invalid-index catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing invalid-index rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var schema, table, idx string
		if scanErr := rows.Scan(&schema, &table, &idx); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf(
			"- %s.%s on %s.%s: INVALID — ignored by the planner but still maintained on every write; usually a crashed CREATE INDEX CONCURRENTLY. Finish with REINDEX INDEX CONCURRENTLY %s, or DROP INDEX if unwanted",
			schema, idx, schema, table, fmt.Sprintf("%s.%s", quoteIdent(schema), quoteIdent(idx))))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate invalid-index rows: %w", err)
	}

	if len(lines) == 0 {
		return "No invalid indexes — all user indexes are usable.", nil
	}
	return fmt.Sprintf("%d invalid index(es):\n%s", len(lines), strings.Join(lines, "\n")), nil
}
