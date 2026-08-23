package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Slow-query-log audit: with slow_query_log OFF or the default
// long_query_time of 10 seconds, slow queries never land anywhere an
// agent can inspect — engine_slow_queries (performance schema /
// sys schema digests) silently sees nothing. The fix is a runtime
// SET GLOBAL, no restart required.

const slowQueryTimeQuietSecs = 5

// slowQueryLogProbe returns the probe reading both settings, or ""
// when unsupported.
func slowQueryLogProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT @@GLOBAL.slow_query_log AS enabled,
       @@GLOBAL.long_query_time AS threshold_secs`
	default:
		return ""
	}
}

// slowQueryLogVerdict classifies the configuration; a log that is on
// with a tight-enough threshold renders "" so reports stay
// actionable.
func slowQueryLogVerdict(enabled bool, thresholdSecs float64) string {
	if !enabled {
		return "WARNING: the slow query log is disabled — slow queries vanish unrecorded and engine_slow_queries has nothing to correlate against. Fix: SET GLOBAL slow_query_log = ON (and set long_query_time, see below)."
	}
	if thresholdSecs >= slowQueryTimeQuietSecs {
		return fmt.Sprintf(
			"WARNING: long_query_time=%g s — the 10 s default hides everything but catastrophic queries; most production incidents live between 0.1 s and 2 s. Fix: SET GLOBAL long_query_time = 1.",
			thresholdSecs)
	}
	return ""
}

// AuditSlowQueryLog renders whether slow queries are actually being
// recorded at a useful threshold; a healthy result is stated
// explicitly.
func (uc *DatabaseUseCase) AuditSlowQueryLog(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := slowQueryLogProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("slow-query-log introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("slow-query-log query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing slow-query-log rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read slow-query-log settings: %w", rerr)
		}
		return "", fmt.Errorf("slow-query-log query returned no rows")
	}

	var rawEnabled, rawThreshold interface{}
	if scanErr := rows.Scan(&rawEnabled, &rawThreshold); scanErr != nil {
		return "", fmt.Errorf("failed to scan slow-query-log settings: %w", scanErr)
	}
	enabled := truthySetting(fmt.Sprintf("%v", rawEnabled))
	threshold, perr := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", rawThreshold)), 64)
	if perr != nil {
		return "", fmt.Errorf("unparseable long_query_time %v: %w", rawThreshold, perr)
	}

	if verdict := slowQueryLogVerdict(enabled, threshold); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Slow query log healthy: enabled, long_query_time=%g s.", threshold), nil
}
