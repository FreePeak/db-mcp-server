package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// effective_cache_size audit: the planner uses this setting to
// estimate how much of a table/index the OS page cache is likely to
// hold when costing index scans. The 4GB default under-assumes on
// modern RAM-heavy hosts, biasing plans toward seq scans that the
// cache would have made cheap. Like random_page_cost this is
// host-dependent advice, so the warning names what to size it against.

const effCacheDefault = "4gb" // PG default; lowercased for comparison

// effectiveCacheProbe returns the probe for the setting, or "" when
// unsupported.
func effectiveCacheProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT COALESCE(current_setting('effective_cache_size'), '') AS ecs`
	default:
		return ""
	}
}

// effectiveCacheVerdict classifies the setting; tuned values render
// "" so reports stay actionable.
func effectiveCacheVerdict(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case v == "":
		return "effective_cache_size is unreadable — verify with SHOW effective_cache_size."
	case v == effCacheDefault:
		return "WARNING: effective_cache_size=4GB — still at the default, which under-assumes the OS page cache on RAM-heavy hosts and biases plans away from index scans. Size it to (total RAM - other processes' working sets), e.g. ALTER SYSTEM SET effective_cache_size='24GB' then SELECT pg_reload_conf(). It's an estimate only — no memory is reserved."
	default:
		return "" // explicitly configured beyond the default
	}
}

// AuditEffectiveCache renders whether the planner's cache assumption
// matches the host's actual memory; a tuned result is stated
// explicitly.
func (uc *DatabaseUseCase) AuditEffectiveCache(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := effectiveCacheProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("effective_cache_size introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("effective_cache_size query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing effective_cache_size rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read effective_cache_size: %w", rerr)
		}
		return "", fmt.Errorf("effective_cache_size query returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan effective_cache_size: %w", scanErr)
	}
	if verdict := effectiveCacheVerdict(raw); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("effective_cache_size=%s — planner cache assumption configured beyond the default.", strings.TrimSpace(raw)), nil
}
