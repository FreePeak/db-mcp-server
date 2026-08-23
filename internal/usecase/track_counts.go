package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// track_counts audit: the flag defaults on, but when it's been
// turned off to shave CPU every pg_stat_* counter freezes at zero —
// autovacuum goes blind (its thresholds come from n_dead_tup /
// n_mod_since_analyze), planner stats go stale, and stats-based
// tooling silently reports zeros instead of failing loudly. The
// failure is invisible precisely because everything still "works".

// trackCountsProbe returns the probe reading the flag, or "" when
// unsupported.
func trackCountsProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('track_counts') AS tc`
	default:
		return ""
	}
}

// trackCountsVerdict classifies the flag; enabled renders "" so
// reports stay actionable.
func trackCountsVerdict(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "on":
		return ""
	case "off":
		return "WARNING: track_counts=off — pg_stat counters are frozen: autovacuum cannot see dead tuples so bloat accumulates untriggered, and every statistics-based diagnostic reads zeros while looking healthy. Fix: ALTER SYSTEM SET track_counts = on then SELECT pg_reload_conf(); the CPU saving is negligible next to the blindness."
	default:
		return "track_counts is empty or unreadable on this platform — verify with SHOW track_counts."
	}
}

// AuditTrackCounts renders whether statistics collection feeds
// autovacuum and diagnostics; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditTrackCounts(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := trackCountsProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("track_counts introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("track_counts query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing track-counts rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read track_counts: %w", rerr)
		}
		return "", fmt.Errorf("track_counts query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan track_counts: %w", scanErr)
	}
	s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if s == "" {
		s = "unreadable"
	}
	if verdict := trackCountsVerdict(s); verdict != "" {
		return verdict, nil
	}
	return "track_counts healthy: on — autovacuum and diagnostics see live statistics.", nil
}
