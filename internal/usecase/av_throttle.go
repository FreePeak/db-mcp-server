package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Autovacuum-throttle audit: the legacy cost budget
// (autovacuum_vacuum_cost_delay=2ms, limit inherited from
// vacuum_cost_limit=200) was calibrated for spinning disks. On modern
// storage with busy write-heavy tables, throttled autovacuum loses the
// race: dead tuples accumulate, bloat grows, and indexes fatten until
// every query pays. Raising the limit (or zeroing the delay) lets
// vacuum keep pace; per-table autovacuum-off is a separate audit.

// avThrottleQuery returns the probe for both cost settings, or ""
// when unsupported.
func avThrottleQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('autovacuum_vacuum_cost_delay') AS delay,
       current_setting('autovacuum_vacuum_cost_limit') AS cap`
	default:
		return ""
	}
}

// parseGUCms reads a GUC time value ("2ms", "0", "100") as
// milliseconds; ok=false when unrecognizable.
func parseGUCms(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, false
	}
	mult := 1.0
	for _, suf := range []struct {
		name string
		f    float64
	}{{"us", 0.001}, {"ms", 1}, {"s", 1000}, {"min", 60000}} {
		if strings.HasSuffix(s, suf.name) {
			mult = suf.f
			s = strings.TrimSuffix(s, suf.name)
			break
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * mult, err == nil
}

// avThrottleVerdict classifies the vacuum budget; an unthrottled or
// raised budget renders "" so reports stay actionable. delayMs<0 or
// limit<=-2 mean unreadable input.
func avThrottleVerdict(delayRaw, limitRaw string) string {
	delayMs, delayOK := parseGUCms(delayRaw)
	limit, limitErr := strconv.Atoi(strings.TrimSpace(limitRaw))
	if !delayOK || limitErr != nil {
		return "Autovacuum cost settings are empty or unreadable — verify with SHOW autovacuum_vacuum_cost_delay."
	}
	if delayMs == 0 || limit >= 500 { // unthrottled, or a budget that keeps pace on modern storage
		return ""
	}
	return fmt.Sprintf("WARNING: autovacuum runs at the legacy spinning-disk budget (cost_delay=%s, cost_limit=%s) — busy write-heavy tables outpace it and bloat accumulates invisibly. Fix: ALTER SYSTEM SET autovacuum_vacuum_cost_limit='2000' (and optionally autovacuum_vacuum_cost_delay='1ms') then SELECT pg_reload_conf(); watch dead-tuple counts after.",
		strings.TrimSpace(delayRaw), strings.TrimSpace(limitRaw))
}

// AuditAVThrottle renders whether autovacuum's cost budget plausibly
// matches modern storage; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditAVThrottle(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := avThrottleQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("autovacuum-throttle introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("autovacuum-throttle query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing autovacuum-throttle rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read autovacuum cost settings: %w", rerr)
		}
		return "", fmt.Errorf("autovacuum-throttle query returned no rows")
	}

	var delayRaw, limitRaw string
	if scanErr := rows.Scan(&delayRaw, &limitRaw); scanErr != nil {
		return "", fmt.Errorf("failed to scan autovacuum cost settings: %w", scanErr)
	}
	if verdict := avThrottleVerdict(delayRaw, limitRaw); verdict != "" {
		return verdict, nil
	}
	return "Autovacuum budget healthy: not pinned at the spinning-disk default.", nil
}
