package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Query row-count preview: wraps a SELECT in COUNT(*) so an agent can
// price a query before fetching rows. Works on every engine (subquery in
// FROM is universal SQL) and bypasses max_rows since only one row returns.

// IsSelectStatement reports whether the statement's leading verb is one the
// COUNT(*) wrap supports: plain SELECT or a WITH CTE.
func IsSelectStatement(query string) bool {
	tokens := sqlWordTokens(stripSQLLiterals(query))
	if len(tokens) == 0 {
		return false
	}
	switch strings.ToUpper(tokens[0]) {
	case "SELECT", "WITH":
		return true
	default:
		return false
	}
}

// CountQueryRows executes SELECT COUNT(*) FROM (<query>) and renders the
// total. Non-SELECT statements are rejected.
func (uc *DatabaseUseCase) CountQueryRows(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}
	if !IsSelectStatement(strings.TrimSpace(stripSQLLiterals(query))) {
		return "", fmt.Errorf("count_only requires a SELECT statement")
	}

	wrapped := fmt.Sprintf("SELECT COUNT(*) AS row_count FROM (%s) AS count_subquery", strings.TrimRight(strings.TrimSpace(query), ";"))
	rows, err := db.Query(ctx, wrapped, params...)
	if err != nil {
		return "", fmt.Errorf("count query failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing count rows: %v", closeErr)
		}
	}()
	out, _, err := renderQueryResults(rows, 1, false, VerbosityFull)
	return out, err
}
