package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// log_checkpoints audit: with the flag off (the default on PG ≤14),
// checkpoint start/finish timing, sync duration, and buffers-written
// counts never reach the logs — exactly the evidence needed to tune
// max_wal_size / checkpoint_timeout after an I/O stall. PG15+
// defaults it on; older engines keep flying blind.

// logCheckpointsProbe returns the probe reading the logging flag, or
// "" when unsupported.
func logCheckpointsProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('log_checkpoints') AS lc`
	default:
		return ""
	}
}

// logCheckpointsVerdict classifies the flag; enabled renders "" so
// reports stay actionable.
func logCheckpointsVerdict(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "on":
		return ""
	case "off":
		return "WARNING: log_checkpoints=off — checkpoint timing, sync duration, and buffers-written counts are invisible, so I/O stalls at checkpoints cannot be diagnosed after the fact. Fix: ALTER SYSTEM SET log_checkpoints = on then SELECT pg_reload_conf(); use the logged durations to tune max_wal_size / checkpoint_timeout. (PG15+ defaults this on.)"
	default:
		return "log_checkpoints is empty or unreadable on this platform — verify with SHOW log_checkpoints."
	}
}

// AuditLogCheckpoints renders whether checkpoint activity reaches
// the logs; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditLogCheckpoints(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := logCheckpointsProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("log_checkpoints introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("log_checkpoints query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing log-checkpoint rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read log_checkpoints: %w", rerr)
		}
		return "", fmt.Errorf("log_checkpoints query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan log_checkpoints: %w", scanErr)
	}
	s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if s == "" {
		s = "unreadable"
	}
	if verdict := logCheckpointsVerdict(s); verdict != "" {
		return verdict, nil
	}
	return "log_checkpoints healthy: on — checkpoint timing and buffer counts reach the logs.", nil
}
