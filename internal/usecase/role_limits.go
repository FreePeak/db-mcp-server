package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Role connection-limit audit: a login role that reaches its CONNECTION
// LIMIT rejects every new session with a confusing error while other
// roles log in fine. list_sessions shows sessions; only pg_roles'
// rolconnlimit compared to live per-role usage reveals who is about to
// start rejecting.

// roleLimitQuery returns the SELECT joining login roles' finite
// connection limits to live session counts, or "" when unsupported.
func roleLimitQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT r.rolname,
       r.rolconnlimit,
       COALESCE(s.n, 0) AS in_use
FROM pg_roles r
LEFT JOIN (
  SELECT usename, COUNT(*) AS n
  FROM pg_stat_activity
  GROUP BY usename
) s ON s.usename = r.rolname
WHERE r.rolcanlogin AND r.rolconnlimit >= 0
ORDER BY r.rolname`
	default:
		return ""
	}
}

// roleLimitLine renders one capped role's usage; roles comfortably
// below their cap render "" so the report stays actionable.
func roleLimitLine(role string, limit, inUse int64) string {
	switch {
	case inUse >= limit:
		return fmt.Sprintf("- %s: AT LIMIT (%d/%d) — rejecting new connections NOW", role, inUse, limit)
	case inUse*5 >= limit*4: // ≥80% of cap
		return fmt.Sprintf("- %s: WARNING %d/%d used — new logins will start failing soon", role, inUse, limit)
	default:
		return ""
	}
}

// ListRoleConnectionLimits renders every capped role at or near its
// connection limit; a clean result is stated explicitly.
func (uc *DatabaseUseCase) ListRoleConnectionLimits(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := roleLimitQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("role-limit introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("role-limit catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing role-limit rows: %v", closeErr)
		}
	}()

	var lines []string
	capped := 0
	for rows.Next() {
		var role string
		var limit, inUse int64
		if scanErr := rows.Scan(&role, &limit, &inUse); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		capped++
		if line := roleLimitLine(role, limit, inUse); line != "" {
			lines = append(lines, line)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate role-limit rows: %w", err)
	}

	if len(lines) == 0 {
		out := "No login roles at risk: no role at its connection limit."
		if capped == 0 {
			out = "No login roles have a finite connection limit set."
		}
		return out, nil
	}
	return fmt.Sprintf("%d of %d capped role(s) at risk:\n%s",
		len(lines), capped, strings.Join(lines, "\n")), nil
}
