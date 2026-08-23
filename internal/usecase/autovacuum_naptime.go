package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// autovacuum_naptime audit: how long autovacuum waits between
// passes (default 60s). Raised values delay every table's cleanup
// cadence, so bloat accumulates between passes on busy tables — and
// because naptime applies per-database, clusters with many databases
// multiply the effective delay per table.

const autovacuumNaptimeQuietSecs = 300

// autovacuumNaptimeProbe returns the probe reading the setting, or
// "" when unsupported.
func autovacuumNaptimeProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('autovacuum_naptime') AS naptime`
	default:
		return ""
	}
}

// parseSecondsSetting parses a GUC seconds value: bare numbers are
// seconds, suffixed forms ("1min", "90s", "2 min") use time.ParseDuration.
func parseSecondsSetting(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// GUCs spell minutes "min"; time.ParseDuration wants "m".
	normalized := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "min", "m")
	if d, err := time.ParseDuration(normalized); err == nil {
		return int(d.Seconds())
	}
	return 0
}

// autovacuumNaptimeVerdict classifies the naptime; at or below the
// quiet threshold renders "" so reports stay actionable.
func autovacuumNaptimeVerdict(secs int) string {
	if secs <= 0 {
		return "autovacuum_naptime is empty or unreadable — verify with SHOW autovacuum_naptime."
	}
	if secs <= autovacuumNaptimeQuietSecs {
		return ""
	}
	return fmt.Sprintf(
		"WARNING: autovacuum_naptime=%s — every table's cleanup cadence is delayed this long between passes, so dead-tuple bloat accumulates on busy tables; the delay multiplies across databases since the setting applies per-database. Fix: ALTER SYSTEM SET autovacuum_naptime = '60s' then SELECT pg_reload_conf().",
		time.Duration(secs)*time.Second)
}

// AuditAutovacuumNaptime renders whether vacuum passes run often
// enough to keep bloat bounded; a healthy result is stated
// explicitly.
func (uc *DatabaseUseCase) AuditAutovacuumNaptime(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := autovacuumNaptimeProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("autovacuum_naptime introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("autovacuum_naptime query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing autovacuum-naptime rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read autovacuum_naptime: %w", rerr)
		}
		return "", fmt.Errorf("autovacuum_naptime query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan autovacuum_naptime: %w", scanErr)
	}
	secs := parseSecondsSetting(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if verdict := autovacuumNaptimeVerdict(secs); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("autovacuum_naptime healthy: %ds — cleanup passes run frequently enough to bound bloat.", secs), nil
}
