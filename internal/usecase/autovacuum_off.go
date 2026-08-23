package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Autovacuum-disabled audit: someone setting autovacuum_enabled=false
// on a big table years ago means dead-tuple bloat and XID-wraparound
// risk accumulate in silence — the wraparound and maintenance audits
// measure the damage, but only the reloptions flag reveals the tables
// where the janitor was fired.

// autovacuumDisabledQuery returns the SELECT for user tables with
// explicitly disabled autovacuum, or "" when unsupported.
func autovacuumDisabledQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT c.relname,
       GREATEST(c.reltuples::bigint, 0) AS row_estimate
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND c.relkind IN ('r', 'p')
  AND c.reloptions IS NOT NULL
  AND array_to_string(c.reloptions, ',') LIKE '%autovacuum_enabled=false%'
ORDER BY c.relname`
	default:
		return ""
	}
}

// ListAutovacuumDisabled renders every user table whose storage
// parameters turn autovacuum off; a clean result is stated explicitly.
func (uc *DatabaseUseCase) ListAutovacuumDisabled(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := autovacuumDisabledQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("autovacuum introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("autovacuum catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing autovacuum rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var name string
		var rowEst int64
		if scanErr := rows.Scan(&name, &rowEst); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf("- %s (~%d row(s)): autovacuum is OFF — dead tuples and frozen-XID age accumulate silently here",
			name, rowEst))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate autovacuum rows: %w", err)
	}

	if len(lines) == 0 {
		return "No user tables have autovacuum disabled.", nil
	}
	out := fmt.Sprintf("%d table(s) with autovacuum explicitly disabled:\n%s\n",
		len(lines), strings.Join(lines, "\n"))
	out += "Re-enable unless intentional: ALTER TABLE <t> SET (autovacuum_enabled = true); then VACUUM (ANALYZE)."
	return strings.TrimRight(out, "\n"), nil
}
