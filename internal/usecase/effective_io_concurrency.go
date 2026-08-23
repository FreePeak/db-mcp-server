package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// effective_io_concurrency audit: the default 1 is calibrated for
// spinning disks where seeking dominates — on SSD/NVMe storage,
// bitmap heap scans lose prefetch parallelism and read large ranges
// one page at a time. Raising it (~200 on SSD) lets the kernel keep
// the device queue full during big scans.

const ioConcurrencyQuiet = 200

// effectiveIOConcurrencyProbe returns the probe reading the setting,
// or "" when unsupported.
func effectiveIOConcurrencyProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('effective_io_concurrency') AS eic`
	default:
		return ""
	}
}

// effectiveIOConcurrencyVerdict classifies the parsed value; at or
// above the SSD-appropriate level renders "" so reports stay
// actionable.
func effectiveIOConcurrencyVerdict(v int) string {
	if v <= 0 {
		return "effective_io_concurrency is 0 or unreadable — prefetching is disabled; verify with SHOW effective_io_concurrency."
	}
	if v >= ioConcurrencyQuiet {
		return ""
	}
	return fmt.Sprintf(
		"WARNING: effective_io_concurrency=%d — the default is calibrated for a spinning disk; on SSD/NVMe bitmap-heap scans prefetch one page at a time and read large ranges serially. Fix: ALTER SYSTEM SET effective_io_concurrency = %d then SELECT pg_reload_conf().",
		v, ioConcurrencyQuiet)
}

// AuditEffectiveIOConcurrency renders whether scan prefetching is
// tuned for modern storage; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditEffectiveIOConcurrency(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := effectiveIOConcurrencyProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("effective_io_concurrency introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("effective_io_concurrency query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing effective-io-concurrency rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read effective_io_concurrency: %w", rerr)
		}
		return "", fmt.Errorf("effective_io_concurrency query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan effective_io_concurrency: %w", scanErr)
	}
	v := 0
	if n, perr := strconv.Atoi(strings.TrimSpace(fmt.Sprintf("%v", raw))); perr == nil {
		v = n
	}
	if verdict := effectiveIOConcurrencyVerdict(v); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("effective_io_concurrency healthy: %d — scans prefetch in parallel on modern storage.", v), nil
}
