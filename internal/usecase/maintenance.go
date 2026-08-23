package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Maintenance suggestions: read engine statistics catalogs and surface
// actionable upkeep — bloat (dead tuples) and stale statistics on
// Postgres, fragmentation on MySQL. Catalog reads only; the suggested
// statements are rendered for review, never executed here.

// maintenanceCatalog returns the engine's maintenance-stats SELECT, or
// "" when unsupported.
func maintenanceCatalog(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT relname AS table_name,
       n_live_tup, n_dead_tup,
       COALESCE(last_analyze::text, last_autoanalyze::text, '') AS last_analyzed
FROM pg_stat_user_tables`
	case "mysql", "mariadb":
		return `SELECT TABLE_NAME AS table_name,
       DATA_LENGTH + INDEX_LENGTH AS bytes_used,
       DATA_FREE AS bytes_free
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'`
	default:
		return ""
	}
}

// ListMaintenance renders per-table upkeep suggestions from engine stats.
func (uc *DatabaseUseCase) ListMaintenance(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := maintenanceCatalog(dbType)
	if q == "" {
		return "", fmt.Errorf("maintenance statistics are not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("maintenance catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing maintenance rows: %v", closeErr)
		}
	}()

	var lines []string
	tables := 0
	for rows.Next() {
		var name string
		switch strings.ToLower(dbType) {
		case "postgres", "postgresql":
			var live, dead int64
			var analyzed interface{}
			if scanErr := rows.Scan(&name, &live, &dead, &analyzed); scanErr != nil {
				continue
			}
			tables++
			if live > 1000 && dead*10 >= live { // ≥10% bloat on non-trivial tables
				lines = append(lines, fmt.Sprintf(
					"- %s: %d dead / %d live tuple(s) — run VACUUM ANALYZE %s",
					name, dead, live, name))
			}
			if analyzed == nil || analyzed == "" {
				lines = append(lines, fmt.Sprintf("- %s: never analyzed — run ANALYZE %s for reliable plans", name, name))
			}
		default: // mysql family
			var used, free interface{}
			if scanErr := rows.Scan(&name, &used, &free); scanErr != nil {
				continue
			}
			tables++
			freeN, freeOK := toInt64(free)
			usedN, usedOK := toInt64(used)
			if freeOK && usedOK && usedN > 1024*1024 && freeN*10 >= usedN { // ≥10% fragmented, >1MB
				lines = append(lines, fmt.Sprintf(
					"- %s: %.1f MB free space in %.1f MB allocated — consider OPTIMIZE TABLE %s",
					name, float64(freeN)/(1024*1024), float64(usedN)/(1024*1024), name))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate maintenance rows: %w", err)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No maintenance flagged across %d table(s): no significant bloat, fragmentation, or missing statistics.", tables), nil
	}
	out := fmt.Sprintf("%d maintenance suggestion(s):\n%s", len(lines), strings.Join(lines, "\n"))
	return out, nil
}

// toInt64 converts a driver value that should be numeric.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case uint64:
		return int64(n), true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case []byte:
		var out int64
		if _, err := fmt.Sscanf(string(n), "%d", &out); err == nil {
			return out, true
		}
	}
	return 0, false
}
