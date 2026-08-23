package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// SQLite busy_timeout audit: the default is 0 ms — any lock contention
// fails immediately with SQLITE_BUSY instead of waiting for the lock
// to clear. It is the companion fix to WAL mode (which still allows
// exactly one writer): with a retry window set, concurrent access
// degrades to brief waits rather than errors.

// busyTimeoutQuery returns the busy-timeout pragma for SQLite, or ""
// when unsupported.
func busyTimeoutQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "sqlite", "sqlite3":
		return `PRAGMA busy_timeout`
	default:
		return ""
	}
}

// busyTimeoutVerdict classifies the retry window; a healthy value
// renders "" so reports stay actionable.
func busyTimeoutVerdict(ms int64) string {
	if ms > 0 {
		return ""
	}
	return "WARNING: busy_timeout=0 — concurrent writers fail immediately with SQLITE_BUSY (database busy) instead of waiting. Fix: PRAGMA busy_timeout=5000 (per-connection; set it in the application DSN/open hook too)."
}

// AuditBusyTimeout renders whether lock contention waits or fails;
// a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditBusyTimeout(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := busyTimeoutQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("busy_timeout introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("busy_timeout pragma failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing busy_timeout rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read busy_timeout: %w", rerr)
		}
		return "", fmt.Errorf("busy_timeout pragma returned no rows")
	}

	var raw string
	if scanErr := rows.Scan(&raw); scanErr != nil {
		return "", fmt.Errorf("failed to scan busy_timeout: %w", scanErr)
	}
	ms, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if perr != nil {
		logger.Error("unparseable busy_timeout %q: %v", raw, perr)
		ms = 0
	}
	if verdict := busyTimeoutVerdict(ms); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf("busy_timeout=%dms: lock contention waits for the lock instead of failing.", ms), nil
}
