package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// default_statistics_target audit: ANALYZE samples this many rows per
// column to build the planner's statistics. The default 100 is fine
// for uniform data but underestimates distinct-value counts and
// skew on wide/skewed columns, producing row-count misestimates that
// cascade into wrong join orders. Raising it trades longer ANALYZE
// runs (and bigger pg_statistic) for better plans.

const statisticsTargetDefault = 100

// statisticsTargetProbe returns the probe for the setting, or ""
// when unsupported.
func statisticsTargetProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('default_statistics_target'), '') AS dst`
	default:
		return ""
	}
}

// statisticsTargetVerdict classifies the setting; tuned values render
// "" so reports stay actionable. Unparseable values read as
// unreadable rather than guessed at.
func statisticsTargetVerdict(raw string) string {
	v := strings.TrimSpace(raw)
	n, err := strconv.Atoi(v)
	switch {
	case err != nil || n <= 0:
		return "default_statistics_target is unreadable — verify with SHOW default_statistics_target."
	case n <= statisticsTargetDefault:
		return "WARNING: default_statistics_target=100 — still at the default, which samples too few rows per column for skewed distributions or high-cardinality keys; the resulting misestimates cascade into bad join orders. Fix for stats-sensitive schemas: ALTER SYSTEM SET default_statistics_target='250' then SELECT pg_reload_conf(); run ANALYZE so existing tables pick up deeper stats (per-column ALTER TABLE … ALTER COLUMN … SET STATISTICS overrides this where needed). Larger values make ANALYZE slower — tune per schema."
	default:
		return "" // explicitly raised beyond the default
	}
}

// AuditStatisticsTarget renders whether ANALYZE's sampling depth is
// configured for the workload; a tuned result is stated explicitly.
func (uc *DatabaseUseCase) AuditStatisticsTarget(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := statisticsTargetProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("default_statistics_target introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("default_statistics_target query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing default_statistics_target rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read default_statistics_target: %w", rerr)
		}
		return "", fmt.Errorf("default_statistics_target query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan default_statistics_target: %w", scanErr)
	}
	if verdict := statisticsTargetVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("default_statistics_target=%s — sampling depth configured beyond the default.", strings.TrimSpace(raw)), nil
}
