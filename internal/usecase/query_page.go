package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Query pagination: window a SELECT into a page and report the total
// matching row count in one call, so an agent can page without
// hand-writing LIMIT/OFFSET arithmetic or a separate COUNT.

// ExecuteQueryPage renders page `page` (1-based) of `pageSize` rows for
// the statement, plus the total matching row count. Non-positive values
// fall back to page 1 / 50 rows.
func (uc *DatabaseUseCase) ExecuteQueryPage(ctx context.Context, dbID, query string, params []interface{}, page, pageSize int) (string, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", 0, fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}
	if !IsSelectStatement(strings.TrimSpace(stripSQLLiterals(query))) {
		return "", 0, fmt.Errorf("paging requires a SELECT statement")
	}

	clean := strings.TrimRight(strings.TrimSpace(query), ";")
	var total int64
	countRows, err := db.Query(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS page_subquery", clean), params...)
	if err == nil && countRows.Next() {
		if scanErr := countRows.Scan(&total); scanErr != nil {
			total = 0
		}
	}
	if countRows != nil {
		if cerr := countRows.Close(); cerr != nil {
			logger.Error("error closing page-count rows: %v", cerr)
		}
	}

	offset := (page - 1) * pageSize
	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT * FROM (%s) AS page_subquery LIMIT %d OFFSET %d", clean, pageSize, offset), params...)
	if err != nil {
		return "", total, fmt.Errorf("paged query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing paged rows: %v", cerr)
		}
	}()
	out, _, err := renderQueryResults(rows, db.MaxRows(), false, VerbosityFull)
	if err != nil {
		return "", total, err
	}
	header := fmt.Sprintf("Page %d (%d rows/page) of %d matching rows:\n\n", page, pageSize, total)
	return header + out, total, nil
}
