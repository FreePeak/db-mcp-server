package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Session observability: list active server sessions and cancel a running
// query, the pg_stat_activity/pg_locks equivalent an agent needs when
// "the database feels slow". Engine-gated: SQLite has no server sessions.

// sessionsQuery returns the engine's active-session catalog SELECT, or ""
// when the engine has no server-side session model.
func sessionsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT pid, usename, application_name, client_addr, state,
wait_event_type, wait_event, state_change, query_start, query
FROM pg_stat_activity WHERE state <> 'idle' ORDER BY query_start`
	case "mysql", "mariadb":
		return `SELECT id, user, host, db, command, time, state, info
FROM information_schema.processlist WHERE command <> 'Sleep' ORDER BY time DESC`
	default:
		return ""
	}
}

// cancelQueryStmt returns the statement that cancels (not terminates) a
// running query on the given session id, and whether the engine supports it.
func cancelQueryStmt(dbType string, sessionID int64) (string, bool) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return fmt.Sprintf("SELECT pg_cancel_backend(%d)", sessionID), true
	case "mysql", "mariadb":
		return fmt.Sprintf("KILL QUERY %d", sessionID), true
	default:
		return "", false
	}
}

// ListActiveSessions renders the engine's currently active sessions.
func (uc *DatabaseUseCase) ListActiveSessions(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := sessionsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("session listing is not supported for engine %q (no server-side session catalog)", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("session catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing rows: %v", closeErr)
		}
	}()
	out, _, err := renderQueryResults(rows, 100, false, VerbosityFull)
	return out, err
}

// CancelQuery cancels a running query on the given session id. Cancel, not
// kill: the connection itself stays alive.
func (uc *DatabaseUseCase) CancelQuery(ctx context.Context, dbID string, sessionID int64) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	stmt, ok := cancelQueryStmt(dbType, sessionID)
	if !ok {
		return "", fmt.Errorf("query cancellation is not supported for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if _, err := db.Exec(ctx, stmt); err != nil {
		return "", fmt.Errorf("cancel failed: %w", err)
	}
	return fmt.Sprintf("Cancel requested for session %d (engine %s). Verify with list_sessions.", sessionID, dbType), nil
}
