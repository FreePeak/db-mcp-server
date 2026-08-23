package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Unpopulated materialized-view audit: a matview created WITH NO DATA
// (or whose initial populate failed) errors on every query until
// someone runs REFRESH MATERIALIZED VIEW — and it looks identical to a
// working view from the outside. Only pg_matviews.ispopulated reveals
// which views are shells.

// unpopulatedMatviewQuery returns the SELECT for materialized views
// that have never been populated, or "" when unsupported.
func unpopulatedMatviewQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT schemaname,
       matviewname
FROM pg_matviews
WHERE NOT ispopulated
ORDER BY schemaname, matviewname`
	default:
		return ""
	}
}

// ListUnpopulatedMatviews renders every materialized view that errors
// on query; a clean result is stated explicitly.
func (uc *DatabaseUseCase) ListUnpopulatedMatviews(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := unpopulatedMatviewQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("materialized-view introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("materialized-view catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing matview rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var schema, name string
		if scanErr := rows.Scan(&schema, &name); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		lines = append(lines, fmt.Sprintf(
			"- %s.%s: UNPOPULATED — every query against it errors until REFRESH MATERIALIZED VIEW %s.%s",
			schema, name, quoteIdent(schema), quoteIdent(name)))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate matview rows: %w", err)
	}

	if len(lines) == 0 {
		return "No unpopulated materialized views — every matview is queryable.", nil
	}
	return fmt.Sprintf("%d unpopulated materialized view(s):\n%s", len(lines), strings.Join(lines, "\n")), nil
}
