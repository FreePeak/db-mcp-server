package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Idle-session listing: when connection_saturation warns, the agent
// needs to know WHO holds the slots. list_sessions deliberately filters
// out PG idle sessions to show activity — this is the complement: the
// connections doing nothing while eating the client ceiling (pool
// leaks, forgotten shells, stuck app instances).

// idleSessionsQuery returns the engine's idle-connection SELECT
// (oldest-idle first), or "" when unsupported.
func idleSessionsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT pid,
       COALESCE(usename, '') AS usename,
       COALESCE(application_name, '') AS application_name,
       COALESCE(client_addr::text, 'local') AS client_addr,
       now() - state_change AS idle_for
FROM pg_stat_activity
WHERE state = 'idle'
ORDER BY state_change`
	case "mysql", "mariadb":
		return `SELECT id,
       user,
       '' AS application_name,
       host,
       time AS idle_for_secs
FROM information_schema.processlist
WHERE command = 'Sleep'
ORDER BY time DESC`
	default:
		return ""
	}
}

// ListIdleSessions renders every idle connection oldest-first so a
// saturated server's slot-holders are visible.
func (uc *DatabaseUseCase) ListIdleSessions(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := idleSessionsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("idle-session introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("idle-session catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing idle-session rows: %v", closeErr)
		}
	}()

	var lines []string
	for rows.Next() {
		var pid any
		var user, appName, addr, idleFor string
		if scanErr := rows.Scan(&pid, &user, &appName, &addr, &idleFor); scanErr != nil {
			continue // unscannable row: skip rather than fail the listing
		}
		idleFor = oneLine(idleFor)
		if strings.HasSuffix(idleFor, "s") && !strings.ContainsAny(idleFor, ":") &&
			!strings.Contains(idleFor, "day") {
			idleFor += " ago" // MySQL returns bare seconds
		}
		lines = append(lines, fmt.Sprintf("pid=%v user=%s app=%q from=%s idle for %s",
			pid, user, appName, addr, idleFor))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate idle-session rows: %w", err)
	}

	if len(lines) == 0 {
		return "No idle sessions.", nil
	}
	out := fmt.Sprintf("%d idle session(s), oldest first:\n%s", len(lines), strings.Join(lines, "\n"))
	if len(lines) >= 10 {
		out += "\nMany idle sessions suggest pool misconfiguration or leaked connections."
	}
	return out, nil
}
