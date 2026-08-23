package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Slow-query-log observability audit: with slow_query_log=OFF (MySQL)
// or log_min_duration_statement=-1 (Postgres), the engine records
// nothing about slow queries — you are blind exactly when "the
// database feels slow" starts. engine_slow_queries reads live digests;
// the log is the durable record that survives restarts and proves what
// ran at 3am.

// slowLogQuery returns the probe for the engine's slow-query logging
// settings, or "" when unsupported.
func slowLogQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return `SELECT COALESCE(@@GLOBAL.slow_query_log, 'OFF') AS log_on,
       COALESCE(@@GLOBAL.long_query_time, 10) AS threshold_secs`
	case "postgres", "postgresql":
		return `SELECT current_setting('log_min_duration_statement') AS threshold_ms`
	default:
		return ""
	}
}

// slowLogVerdict classifies MySQL/MariaDB slow-log configuration;
// disabled logging is a warning regardless of threshold.
func slowLogVerdict(logOn string, thresholdSecs float64) string {
	if !strings.EqualFold(strings.TrimSpace(logOn), "ON") {
		return "WARNING: slow query log is OFF — SET GLOBAL slow_query_log=ON with long_query_time around 1 so slow statements leave evidence."
	}
	if thresholdSecs > 5 {
		return fmt.Sprintf("Slow query log is ON but long_query_time=%.0fs is high — queries up to that age never get logged; consider ~1s.", thresholdSecs)
	}
	return fmt.Sprintf("Slow query log healthy: ON with long_query_time=%.1fs.", thresholdSecs)
}

// pgSlowLogVerdict classifies Postgres log_min_duration_statement;
// -1 means statement durations are never logged.
func pgSlowLogVerdict(thresholdMs string) string {
	ms, err := strconv.Atoi(strings.TrimSpace(thresholdMs))
	if err != nil || ms < 0 {
		return "WARNING: log_min_duration_statement=-1 — Postgres logs no slow statements; set it (e.g. 1000ms) to keep evidence."
	}
	return fmt.Sprintf("Statement-duration logging healthy: log_min_duration_statement=%dms.", ms)
}

// AuditSlowLog renders whether the engine would even record a slow
// query; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditSlowLog(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := slowLogQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("slow-log introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("slow-log settings query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing slow-log rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read slow-log settings: %w", rerr)
		}
		return "", fmt.Errorf("slow-log settings query returned no rows")
	}

	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		var thresholdMs string
		if scanErr := rows.Scan(&thresholdMs); scanErr != nil {
			return "", fmt.Errorf("failed to scan slow-log settings: %w", scanErr)
		}
		return pgSlowLogVerdict(thresholdMs), nil
	default:
		var logOn string
		var thresholdSecs float64
		if scanErr := rows.Scan(&logOn, &thresholdSecs); scanErr != nil {
			return "", fmt.Errorf("failed to scan slow-log settings: %w", scanErr)
		}
		return slowLogVerdict(logOn, thresholdSecs), nil
	}
}
