package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// PostgreSQL shared_buffers sizing audit: the default is a famously
// undersized 128MB. When the pool holds only a sliver of the database,
// every cold read hits disk and hit ratios suffer no matter how much
// RAM the host has. The classic fix is ~25% of host RAM (requires a
// restart). This is the PG counterpart of the MySQL buffer_pool audit.

const sharedBuffersWarnRatio = 25 // % of database size the pool must hold

// sharedBuffersQuery returns pool size and current-database size in one
// round trip, or "" when unsupported.
func sharedBuffersQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT 'pool' AS k,
       pg_size_pretty(current_setting('shared_buffers')::bigint * 8192) AS v
UNION ALL
SELECT 'data', pg_size_pretty(pg_database_size(current_database()))`
	default:
		return ""
	}
}

// parsePrettySize parses pg_size_pretty output ("128 MB", "20 GB",
// "512 kB", plain bytes) back to bytes.
func parsePrettySize(s string) int64 {
	f := strings.Fields(strings.TrimSpace(s))
	if len(f) == 0 {
		return 0
	}
	n, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	mult := float64(1)
	if len(f) > 1 {
		switch strings.ToLower(f[1]) {
		case "bytes":
			mult = 1
		case "kb":
			mult = 1024
		case "mb":
			mult = 1024 * 1024
		case "gb":
			mult = 1024 * 1024 * 1024
		case "tb":
			mult = 1024 * 1024 * 1024 * 1024
		}
	}
	return int64(n * mult)
}

// sharedBuffersVerdict classifies the pool against the database size; a
// pool that plausibly covers the hot set renders "" so reports stay
// actionable.
func sharedBuffersVerdict(poolBytes, dataBytes int64) string {
	if poolBytes <= 0 || dataBytes <= 0 {
		return ""
	}
	pct := 100 * poolBytes / dataBytes
	if pct >= sharedBuffersWarnRatio {
		return ""
	}
	return fmt.Sprintf("WARNING: shared_buffers=%s holds only ~%d%% of your %s database — cold reads hit disk and hit ratios will suffer. Fix: size to ~25%% of host RAM, e.g. ALTER SYSTEM SET shared_buffers = '4GB' then restart (this one requires a restart, not just reload).",
		humanBytes(int64(poolBytes)), pct, humanBytes(dataBytes))
}

// AuditSharedBuffers renders whether the shared buffer pool plausibly
// fits the working set; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditSharedBuffers(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := sharedBuffersQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("shared_buffers introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("shared-buffers query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing shared-buffers rows: %v", closeErr)
		}
	}()

	var pool, data int64
	parse := func(s string) int64 {
		n := parsePrettySize(s)
		if n == 0 {
			logger.Error("unparseable shared-buffers value %q", s)
		}
		return n
	}
	for rows.Next() {
		var k, v string
		if scanErr := rows.Scan(&k, &v); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		switch strings.TrimSpace(k) {
		case "pool":
			pool = parse(v)
		case "data":
			data = parse(v)
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return "", fmt.Errorf("failed to iterate shared-buffers rows: %w", rerr)
	}
	if verdict := sharedBuffersVerdict(pool, data); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("Shared buffers healthy: %s pool vs %s database (≥%d%% coverage).",
		humanBytes(int64(pool)), humanBytes(data), sharedBuffersWarnRatio), nil
}
