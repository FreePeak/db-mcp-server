package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// log_lock_waits audit: with the default 'off', any lock wait longer
// than deadlock_timeout is invisible in the logs — the lock_waits
// action shows only what blocks *right now*, and after the incident
// there is no durable evidence of who blocked whom or how long.
// Turning it on costs nothing until a wait actually happens.

// logLockWaitsProbe returns the probe reading the logging flag, or
// "" when unsupported.
func logLockWaitsProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('log_lock_waits') AS llw`
	default:
		return ""
	}
}

// logLockWaitsVerdict classifies the flag; enabled renders "" so
// reports stay actionable.
func logLockWaitsVerdict(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "on":
		return ""
	case "off":
		return "WARNING: log_lock_waits=off — lock waits exceeding deadlock_timeout are never logged. lock_waits shows only current blocking; once the incident passes there's no durable evidence of who blocked whom or how long. Fix: ALTER SYSTEM SET log_lock_waits = on then SELECT pg_reload_conf(); waits are logged against deadlock_timeout (default 1s)."
	default:
		return "log_lock_waits is empty or unreadable on this platform — verify with SHOW log_lock_waits."
	}
}

// AuditLogLockWaits renders whether lock-wait evidence reaches the
// logs; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditLogLockWaits(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := logLockWaitsProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("log_lock_waits introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("log_lock_waits query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing log-lock-waits rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read log_lock_waits: %w", rerr)
		}
		return "", fmt.Errorf("log_lock_waits query returned no rows")
	}

	var raw interface{}
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan log_lock_waits: %w", scanErr)
	}
	s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	if s == "" {
		s = "unreadable"
	}
	if verdict := logLockWaitsVerdict(s); verdict != "" {
		return verdict, nil
	}
	return "log_lock_waits healthy: on — waits past deadlock_timeout reach the logs for post-incident analysis.", nil
}
