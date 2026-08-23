package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Cross-database table copy: read every row from a source database and
// insert it into the same-named table on a destination database inside
// one transaction — seeding staging from prod, backfilling analytics.
// The target table must already exist (create it via migrations or DDL
// first); column lists come from the source catalog so both sides stay
// explicit.

const copyBatchSize = 500

// CopyTable transfers all rows of table from srcDB to dstDB. Values are
// passed through as driver values; cross-engine type fidelity is
// best-effort (same-engine copies are exact).
func (uc *DatabaseUseCase) CopyTable(ctx context.Context, srcDB, dstDB, table string) (string, error) {
	if !isPlainIdentifier(table) {
		return "", fmt.Errorf("invalid table name %q", table)
	}
	if srcDB == dstDB {
		return "", fmt.Errorf("source and destination must differ")
	}

	// Column list from the source catalog keeps inserts explicit and
	// order-stable regardless of physical column order differences.
	desc, err := uc.DescribeTable(ctx, srcDB, table)
	if err != nil {
		return "", fmt.Errorf("failed to describe source %s.%s: %w", srcDB, table, err)
	}
	colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing to copy
	var cols []string
	for _, cr := range colsRaw {
		name := ""
		for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
			if v, ok := cr[k].(string); ok && v != "" {
				name = v
				break
			}
		}
		if name != "" && isPlainIdentifier(name) {
			cols = append(cols, name)
		}
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("no columns discovered for source %s.%s", srcDB, table)
	}

	src, err := uc.repo.GetDatabase(srcDB)
	if err != nil {
		return "", fmt.Errorf("failed to get source database: %w", err)
	}
	dst, err := uc.repo.GetDatabase(dstDB)
	if err != nil {
		return "", fmt.Errorf("failed to get destination database: %w", err)
	}

	rows, err := src.Query(ctx,
		fmt.Sprintf("SELECT %s FROM %s", quoteIdentList(cols), quoteIdent(table)))
	if err != nil {
		return "", fmt.Errorf("failed to read source rows: %w", err)
	}
	type batch [][]any
	var batches []batch
	cur := batch{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if scanErr := rows.Scan(ptrs...); scanErr != nil {
			if cerr := rows.Close(); cerr != nil {
				logger.Error("error closing copy rows: %v", cerr)
			}
			return "", fmt.Errorf("failed to scan source row: %w", scanErr)
		}
		cur = append(cur, vals)
		if len(cur) >= copyBatchSize {
			batches = append(batches, cur)
			cur = batch{}
		}
	}
	if cerr := rows.Close(); cerr != nil {
		logger.Error("error closing copy rows: %v", cerr)
	}
	if rerr := rows.Err(); rerr != nil {
		return "", fmt.Errorf("failed to iterate source rows: %w", rerr)
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}

	colList := quoteIdentList(cols)
	tx, err := dst.Begin(ctx, &domain.TxOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start destination transaction: %w", err)
	}
	inserted := 0
	for _, b := range batches {
		var sb strings.Builder
		args := make([]any, 0, len(b)*len(cols))
		for _, rowVals := range b {
			sb.WriteString(",(")
			for i, v := range rowVals {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString("?")
				args = append(args, v)
			}
			sb.WriteString(")")
		}
		stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
			quoteIdent(table), colList, strings.TrimPrefix(sb.String(), ","))
		if _, execErr := tx.Exec(ctx, stmt, args...); execErr != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.Error("copy rollback failed: %v", rbErr)
			}
			return "", fmt.Errorf("copy into %s.%s failed, nothing committed: %w", dstDB, table, execErr)
		}
		inserted += len(b)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit copy into %s.%s: %w", dstDB, table, err)
	}
	return fmt.Sprintf("Copied %d row(s), %d column(s) from %s.%s to %s.%s.",
		inserted, len(cols), srcDB, table, dstDB, table), nil
}
