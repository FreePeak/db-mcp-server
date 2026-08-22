package usecase

import (
	"context"

	"fmt"
	"github.com/FreePeak/db-mcp-server/internal/domain"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Script execution: run several statements atomically — one transaction,
// stop on first error, roll back everything. The migration-shaped
// alternative to N separate execute calls with no atomicity.

// splitScript breaks a script into statements on semicolons that sit
// outside single- or double-quoted strings. Empty/trailing fragments are
// dropped.
func splitScript(script string) []string {
	var stmts []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range script {
		switch {
		case inQuote != 0:
			cur.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
		case r == '\'' || r == '"':
			inQuote = r
			cur.WriteRune(r)
		case r == ';':
			stmts = append(stmts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if tail := strings.TrimSpace(cur.String()); tail != "" {
		stmts = append(stmts, tail)
	}
	out := stmts[:0]
	for _, s := range stmts {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ExecuteScript runs every statement in the script inside one transaction.
// On success it reports per-statement results; any failure rolls back all
// prior statements and names the offender by position.
func (uc *DatabaseUseCase) ExecuteScript(ctx context.Context, dbID, script string) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() {
		return "", fmt.Errorf("database %q is configured as read-only; scripts are not allowed", dbID)
	}
	stmts := splitScript(script)
	if len(stmts) == 0 {
		return "", fmt.Errorf("script contains no statements")
	}

	tx, err := db.Begin(ctx, &domain.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	results := make([]string, 0, len(stmts))
	for i, stmt := range stmts {
		res, execErr := tx.Exec(ctx, stmt)
		if execErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.Error("script rollback after statement %d failed: %v", i+1, rbErr)
				return "", fmt.Errorf("statement %d failed (%v) AND rollback failed: %w", i+1, execErr, rbErr)
			}
			return "", fmt.Errorf("statement %d failed, transaction rolled back: %w", i+1, execErr)
		}
		results = append(results, fmt.Sprintf("%d: %s", i+1, scriptResultSummary(res)))
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit script: %w", err)
	}
	return fmt.Sprintf("Script committed: %d statement(s) executed.\n%s",
		len(stmts), strings.Join(results, "\n")), nil
}

// scriptResultSummary renders an Exec result compactly for script output.
func scriptResultSummary(res domain.Result) string {
	rows, err := res.RowsAffected()
	if err != nil {
		return "ok"
	}
	return fmt.Sprintf("ok (%d row(s) affected)", rows)
}
