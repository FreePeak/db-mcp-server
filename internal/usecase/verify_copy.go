package usecase

import (
	"context"
	"fmt"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Post-copy reconciliation: compare live row counts between source and
// destination so a copy is verified, not assumed. Count-only — no rows
// are read.

// VerifyCopy reports whether srcDB.table and dstDB.table hold the same
// number of rows, with both counts shown on mismatch.
func (uc *DatabaseUseCase) VerifyCopy(ctx context.Context, srcDB, dstDB, table string) (string, error) {
	if !isPlainIdentifier(table) {
		return "", fmt.Errorf("invalid table name %q", table)
	}
	if srcDB == dstDB {
		return "", fmt.Errorf("source and destination must differ")
	}
	src, err := uc.repo.GetDatabase(srcDB)
	if err != nil {
		return "", fmt.Errorf("failed to get source database: %w", err)
	}
	dst, err := uc.repo.GetDatabase(dstDB)
	if err != nil {
		return "", fmt.Errorf("failed to get destination database: %w", err)
	}

	srcN, err := countTableRows(ctx, src, table)
	if err != nil {
		return "", fmt.Errorf("failed to count source %s.%s: %w", srcDB, table, err)
	}
	dstN, err := countTableRows(ctx, dst, table)
	if err != nil {
		return "", fmt.Errorf("failed to count destination %s.%s: %w", dstDB, table, err)
	}
	if srcN == dstN {
		return fmt.Sprintf("Verified %s: %d row(s) match between %s and %s.",
			table, srcN, srcDB, dstDB), nil
	}
	return fmt.Sprintf("MISMATCH for %s: %s has %d row(s), %s has %d row(s) (delta %+d).",
		table, srcDB, srcN, dstDB, dstN, dstN-srcN), nil
}

// countTableRows runs a validated COUNT(*) against one table.
func countTableRows(ctx context.Context, db domain.Database, table string) (int64, error) {
	rows, err := db.Query(ctx, "SELECT COUNT(*) FROM "+quoteIdent(table))
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing verify rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var n int64
	err = rows.Scan(&n)
	return n, err
}
