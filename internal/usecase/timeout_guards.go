package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// PostgreSQL timeout-guardrail audit: statement_timeout and
// idle_in_transaction_session_timeout both default to 0 — unlimited.
// long_transactions and cancel_query are reactive tools; without
// these guards the engine never stops a runaway statement or a
// forgotten transaction on its own, so locks are held until a human
// notices. Both fixes are runtime SETs, no restart required.

// timeoutGuardsProbe returns the probe reading both GUCs, or ""
// when unsupported.
func timeoutGuardsProbe(dbType string) string {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return `SELECT current_setting('statement_timeout') AS statement_timeout_ms,
       current_setting('idle_in_transaction_session_timeout') AS idle_tx_timeout_ms`
	default:
		return ""
	}
}

// timeoutGuardsVerdict classifies both guards; both-set renders ""
// so reports stay actionable. Values are milliseconds; 0 = unlimited.
func timeoutGuardsVerdict(stmtMs, idleTxMs int64) string {
	var lines []string
	if stmtMs <= 0 {
		lines = append(lines,
			"WARNING: statement_timeout is unset — a runaway SELECT holds its locks and connection until a human cancels it. Fix: ALTER SYSTEM SET statement_timeout = '60s' (or SET per role/session).")
	}
	if idleTxMs <= 0 {
		lines = append(lines,
			"WARNING: idle_in_transaction_session_timeout is unset — a forgotten open transaction blocks vacuum and holds locks indefinitely. Fix: ALTER SYSTEM SET idle_in_transaction_session_timeout = '5min'.")
	}
	return strings.Join(lines, "\n")
}

// AuditTimeoutGuards renders which runaway guards the engine lacks;
// a fully-guarded result is stated explicitly.
func (uc *DatabaseUseCase) AuditTimeoutGuards(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	q := timeoutGuardsProbe(dbType)
	if q == "" {
		return "", fmt.Errorf("timeout-guard introspection is not available for engine %q", dbType)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	rows, err := db.Query(ctx, q)
	if err != nil {
		return "", fmt.Errorf("timeout-guard query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing timeout-guard rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		if rerr := rows.Err(); rerr != nil {
			return "", fmt.Errorf("failed to read timeout guards: %w", rerr)
		}
		return "", fmt.Errorf("timeout-guard query returned no rows")
	}

	var rawStmt, rawIdle interface{}
	if scanErr := rows.Scan(&rawStmt, &rawIdle); scanErr != nil {
		return "", fmt.Errorf("failed to scan timeout guards: %w", scanErr)
	}
	parse := func(v interface{}, name string) (int64, error) {
		n, perr := strconv.ParseInt(strings.TrimSpace(fmt.Sprintf("%v", v)), 10, 64)
		if perr != nil || n < 0 {
			return 0, fmt.Errorf("unparseable %s %v", name, v)
		}
		return n, nil
	}
	stmtMs, err := parse(rawStmt, "statement_timeout")
	if err != nil {
		return "", err
	}
	idleTxMs, err := parse(rawIdle, "idle_in_transaction_session_timeout")
	if err != nil {
		return "", err
	}

	if verdict := timeoutGuardsVerdict(stmtMs, idleTxMs); verdict != "" {
		return verdict, nil
	}
	return fmt.Sprintf(
		"Timeout guards healthy: statement_timeout=%d ms, idle_in_transaction_session_timeout=%d ms.",
		stmtMs, idleTxMs), nil
}
