package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// SQLite WAL-mode audit: the default rollback journal (delete mode)
// blocks readers while a write is in flight — under agent-driven
// concurrent access that surfaces as SQLITE_BUSY storms. WAL mode
// lets readers and one writer proceed concurrently and is one
// persistent pragma away.

// walModeQuery returns the journal-mode pragma for SQLite, or "" when
// unsupported.
func walModeQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "sqlite", "sqlite3":
		return `PRAGMA journal_mode`
	default:
		return ""
	}
}

// walModeVerdict classifies the journal mode against concurrent
// read/write expectations.
func walModeVerdict(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "wal":
		return "WAL mode healthy: readers and one writer proceed concurrently."
	case "memory", "":
		return "Memory journal mode: this is an in-memory database — WAL does not apply; concurrency is bounded by a single connection."
	default:
		return fmt.Sprintf("Journal mode %q blocks readers during writes — under concurrent access expect SQLITE_BUSY (database busy) errors. Enable with: PRAGMA journal_mode=WAL (persistent; applies to future connections too).", strings.TrimSpace(mode))
	}
}

// AuditWALMode renders whether the SQLite database supports concurrent
// readers alongside writers; a healthy result is stated explicitly.
func (uc *DatabaseUseCase) AuditWALMode(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := walModeQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("journal-mode introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("journal-mode pragma failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing journal-mode rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read journal mode: %w", rerr)
		}
		return "", fmt.Errorf("journal-mode pragma returned no rows")
	}

	var mode string
	if scanErr := rows.Scan(&mode); scanErr != nil {
		return "", fmt.Errorf("failed to scan journal mode: %w", scanErr)
	}
	return walModeVerdict(mode), nil
}
