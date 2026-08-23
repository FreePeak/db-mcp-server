package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// mustGetDBType fetches the engine type; sampling falls back to RANDOM()
// when the lookup fails.
func mustGetDBType(repo domain.DatabaseRepository, dbID string) string {
	if t, err := repo.GetDatabaseType(dbID); err == nil {
		return t
	}
	return ""
}

// Random sampling: pull N arbitrary rows from a statement without
// hand-writing engine-specific random ordering (random() vs RAND() vs
// DBMS_RANDOM.VALUE).

// randomOrderBy returns the ORDER BY expression for unbiased sampling on
// the given engine.
func randomOrderBy(dbType string) string {
	switch strings.ToLower(dbType) {
	case "mysql", "mariadb":
		return "RAND()"
	case "oracle":
		return "DBMS_RANDOM.VALUE"
	default: // postgres, sqlite, and friends
		return "RANDOM()"
	}
}

// ExecuteQuerySample renders N randomly ordered rows of the statement.
func (uc *DatabaseUseCase) ExecuteQuerySample(ctx context.Context, dbID, query string, params []interface{}, n int) (string, error) {
	if n <= 0 {
		n = 10
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	if db.IsReadOnly() && IsWriteStatement(query) {
		return "", fmt.Errorf("database %q is configured as read-only; write statements are not allowed via queries", dbID)
	}
	if !IsSelectStatement(strings.TrimSpace(stripSQLLiterals(query))) {
		return "", fmt.Errorf("sampling requires a SELECT statement")
	}

	clean := strings.TrimRight(strings.TrimSpace(query), ";")
	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT * FROM (%s) AS sample_sub ORDER BY %s LIMIT %d",
			clean, randomOrderBy(mustGetDBType(uc.repo, dbID)), n), params...)
	if err != nil {
		return "", fmt.Errorf("sample query failed: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing sample rows: %v", cerr)
		}
	}()
	out, _, err := renderQueryResults(rows, n, false, VerbosityFull)
	return out, err
}
