package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// healthProvider is implemented by repository adapters that can surface
// connection-pool statistics without changing the domain.Database contract.
type healthProvider interface {
	HealthStats() map[string]interface{}
}

// pingProvider is implemented by adapters that can probe liveness.
type pingProvider interface {
	Ping(ctx context.Context) error
}

// HealthCheck reports connectivity, Go connection-pool pressure, and
// best-effort engine-level statistics for one database. Engine queries
// never fail the check — they degrade to notes in the payload.
func (uc *DatabaseUseCase) HealthCheck(ctx context.Context, dbID string) (map[string]interface{}, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	result := map[string]interface{}{
		"database":   dbID,
		"healthy":    true,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	}

	if pp, ok := db.(pingProvider); ok {
		start := time.Now()
		if perr := pp.Ping(ctx); perr != nil {
			result["healthy"] = false
			result["error"] = perr.Error()
			return result, nil
		}
		result["ping_ms"] = float64(time.Since(start).Microseconds()) / 1000.0
	}

	if hp, ok := db.(healthProvider); ok {
		for k, v := range hp.HealthStats() {
			result[k] = v
		}
		// Trend bookkeeping: every check extends the rolling sample.
		if open, ok2 := result["pool_open_connections"].(int); ok2 {
			maxOpen := 0
			if m, ok3 := result["pool_max_open"].(int); ok3 {
				maxOpen = m
			}
			uc.RecordHealthSample(dbID, open, maxOpen)
		}
	}

	// Governance visibility: recent PII redactions for this database.
	if events := uc.GetMaskingAudit(dbID); len(events) > 0 {
		result["masking_events_recent"] = len(events)
		result["masking_events_last"] = events[len(events)-1]
	}

	dbType, typeErr := uc.repo.GetDatabaseType(dbID)
	if typeErr != nil {
		return result, nil
	}
	for k, v := range uc.engineHealthStats(ctx, dbID, strings.ToLower(dbType)) {
		result[k] = v
	}
	return result, nil
}

// engineHealthStats gathers engine-specific indicators; every failure is
// reported as a note rather than propagated.
func (uc *DatabaseUseCase) engineHealthStats(ctx context.Context, dbID, dbType string) map[string]interface{} {
	stats := map[string]interface{}{}

	var query func(string) string
	query = func(sql string) string {
		out, err := uc.ExecuteQuery(ctx, dbID, sql, nil)
		if err != nil {
			stats["engine_stats_error"] = err.Error()
			return ""
		}
		return firstNumericField(out)
	}

	switch dbType {
	case "postgres", "timescale", "timescaledb":
		// Cache hit ratio percentage for the current database.
		if v := query(`SELECT round(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0)) AS ratio FROM pg_stat_database WHERE datname = current_database()`); v != "" {
			stats["buffer_cache_hit_ratio_pct"] = v
		}
	case "mysql":
		// InnoDB buffer pool read efficiency.
		if v := query(`SELECT round(100.0 * (1 - variable_value / NULLIF((SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'Innodb_buffer_pool_read_requests'), 0)), 2) AS ratio FROM performance_schema.global_status WHERE variable_name = 'Innodb_buffer_pool_reads'`); v != "" {
			stats["buffer_cache_miss_ratio_pct"] = v
		}
	default:
		// SQLite/Oracle: no portable equivalent surfaced yet.
	}
	return stats
}

// firstNumericField pulls the first standalone number out of formatted
// query output (see formatQueryResults).
func firstNumericField(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if isNumericField(line) {
			return line
		}
	}
	return ""
}
