package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Timeout-guardrail audit: per-call MCP deadlines protect one client,
// but if the engine itself has no statement_timeout / idle-in-
// transaction ceiling, a runaway query from ANY client runs until
// completion. Reads the engine's own configuration and names the
// missing guardrails so an operator can set them before an incident.

// guardrailsQuery returns the engine's runaway-query settings SELECT
// rendered as (name, setting_ms) rows, or "" when unsupported.
func guardrailsQuery(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT name, setting
FROM pg_settings
WHERE name IN ('statement_timeout', 'idle_in_transaction_session_timeout')`
	case "mysql", "mariadb":
		return `SELECT 'max_execution_time' AS name,
       CAST(@@max_execution_time AS CHAR) AS setting`
	default:
		return ""
	}
}

// CheckTimeoutGuardrails renders each engine-side timeout setting with
// an explicit UNPROTECTED flag for zeros, plus an overall verdict.
func (uc *DatabaseUseCase) CheckTimeoutGuardrails(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := guardrailsQuery(dbType)
	if q == "" {
		return "", fmt.Errorf("timeout-guardrail introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("guardrail catalog query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing guardrail rows: %v", closeErr)
		}
	}()

	var lines []string
	unprotected := 0
	for rows.Next() {
		var name, setting string
		if scanErr := rows.Scan(&name, &setting); scanErr != nil {
			continue // unscannable row: skip rather than fail the audit
		}
		ms := strings.TrimSpace(setting)
		if ms == "0" || ms == "" || ms == "<unset>" {
			lines = append(lines, fmt.Sprintf("- %s: UNPROTECTED (no limit — any client's runaway query runs until completion)", name))
			unprotected++
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s ms", name, ms))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate guardrail rows: %w", err)
	}
	if len(lines) == 0 {
		return "No timeout-guardrail settings readable.", nil
	}

	verdict := "Engine-side guardrails in place."
	if unprotected > 0 {
		verdict = fmt.Sprintf("%d of %d guardrail(s) UNPROTECTED — consider setting them server-side.", unprotected, len(lines))
	}
	return strings.Join(append(lines, verdict), "\n"), nil
}
