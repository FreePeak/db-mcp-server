package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Unused index detection: indexes whose scan counters sit at (or near)
// zero are write-tax with no read benefit. Catalog-backed on engines
// that track usage; explicit unsupported elsewhere.

// unusedIndexQuery returns the engine's unused-index catalog SELECT
// (parameterized by minimum scan count), or "" when unsupported.
func unusedIndexQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT schemaname, relname, indexrelname, idx_scan, pg_size_pretty(pg_relation_size(indexrelid))
FROM pg_stat_user_indexes
WHERE idx_scan < $1 AND NOT indisunique
ORDER BY pg_relation_size(indexrelid) DESC`
	case "mysql", "mariadb":
		return `SELECT object_schema, object_name, index_name
FROM sys.schema_unused_indexes`
	default:
		return ""
	}
}

// ListUnusedIndexes renders indexes the engine has barely (or never)
// scanned. minScans bounds "unused": an index needs fewer than this many
// scans since stats reset to qualify.
func (uc *DatabaseUseCase) ListUnusedIndexes(ctx context.Context, dbID string, minScans int) (string, error) {
	if minScans <= 0 {
		minScans = 100 // default: anything under 100 scans is suspect
	}
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := unusedIndexQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("index usage statistics are not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var args []any
	if strings.Contains(q, "$1") {
		args = append(args, minScans)
	}
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return "", fmt.Errorf("usage-statistics query failed (is the extension/schema installed?): %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing unused-index rows: %v", closeErr)
		}
	}()

	var b strings.Builder
	count := 0
	for rows.Next() {
		vals := make([]any, 5)
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("failed to scan usage row: %w", err)
		}
		// MySQL returns 3 columns; trailing scans stay nil.
		line := fmt.Sprintf("- %v.%v: index %v", vals[0], vals[1], vals[2])
		if vals[3] != nil {
			line += fmt.Sprintf(", scans: %v", vals[3])
		}
		if vals[4] != nil {
			line += fmt.Sprintf(", size: %v", vals[4])
		}
		b.WriteString(line + "\n")
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate usage rows: %w", err)
	}
	if count == 0 {
		return fmt.Sprintf("No unused indexes found (threshold: fewer than %d scans).", minScans), nil
	}
	out := fmt.Sprintf("%d potentially unused index(es) — verify workload before dropping:\n%s", count, b.String())
	return strings.TrimRight(out, "\n"), nil
}
