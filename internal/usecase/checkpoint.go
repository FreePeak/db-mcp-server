package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Checkpoint pressure: checkpoints happen on schedule (timed) or
// because max_wal_size was hit (requested). When requested approaches
// timed, every write burst pays in latency spikes from aggressive
// checkpointing — the classic "writes stall periodically" tuning miss.
// PG17 moved the counters to pg_stat_checkpointer; older versions keep
// them in pg_stat_bgwriter.

// checkpointQueries returns candidate SELECTs in preference order:
// modern view first, legacy fallback second. Empty for unsupported
// engines.
func checkpointQueries(dbType string) []string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return []string{
			`SELECT COALESCE(checkpoints_timed::bigint, 0) AS timed,
       COALESCE(checkpoints_req::bigint, 0) AS requested
FROM pg_stat_checkpointer`,
			`SELECT COALESCE(checkpoints_timed::bigint, 0) AS timed,
       COALESCE(checkpoints_req::bigint, 0) AS requested
FROM pg_stat_bgwriter`,
		}
	default:
		return nil
	}
}

// checkpointVerdict classifies the timed/requested split; ≥20%
// requested means WAL size forces checkpoints instead of time.
func checkpointVerdict(timed, requested int64) string {
	total := timed + requested
	if total == 0 {
		return "No checkpoints recorded yet — server recently started or barely written to."
	}
	if requested*5 >= total { // requested >= 20% of all checkpoints
		return fmt.Sprintf("Checkpoint PRESSURE: %d of %d checkpoints (%.0f%%) were forced by hitting max_wal_size rather than by time — writes pay latency spikes; raise max_wal_size and check checkpoint_completion_target.",
			requested, total, float64(requested)/float64(total)*100)
	}
	return fmt.Sprintf("Checkpoints healthy: %d of %d (%.1f%%) requested; the rest run on schedule.",
		requested, total, float64(requested)/float64(total)*100)
}

// CheckCheckpointPressure renders the timed/requested checkpoint split,
// trying the modern view first and falling back to the legacy one.
func (uc *DatabaseUseCase) CheckCheckpointPressure(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	templates := checkpointQueries(dbType)
	if len(templates) == 0 {
		return "", fmt.Errorf("checkpoint introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var timed, requested int64
	scanned := false
	var lastErr error
	for _, q := range templates {
		rows, err := db.Query(ctx, q)
		if err != nil {
			lastErr = err // e.g. pre-PG17 without pg_stat_checkpointer
			continue
		}
		if rows.Next() {
			if scanErr := rows.Scan(&timed, &requested); scanErr != nil {
				if cerr := rows.Close(); cerr != nil {
					logger.Error("error closing checkpoint rows: %v", cerr)
				}
				lastErr = scanErr
				continue
			}
			scanned = true
		}
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing checkpoint rows: %v", cerr)
		}
		break
	}
	if !scanned {
		if lastErr != nil {
			return "", fmt.Errorf("checkpoint catalog query failed: %w", lastErr)
		}
		return "", fmt.Errorf("checkpoint counters returned no rows")
	}
	return checkpointVerdict(timed, requested), nil
}
