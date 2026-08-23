package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Table profiling: one call renders per-column row count, NULL count,
// distinct count, and min/max — selectivity and nullability answers
// without hand-writing aggregate queries.

// ProfileTable scans every column of table with COUNT/SUM aggregates in
// a single pass per column (bounded by one query each) and renders the
// summary sorted by column name.
func (uc *DatabaseUseCase) ProfileTable(ctx context.Context, dbID, table string) (string, error) {
	if !isPlainIdentifier(table) {
		return "", fmt.Errorf("invalid table name %q", table)
	}
	desc, err := uc.DescribeTable(ctx, dbID, table)
	if err != nil {
		return "", fmt.Errorf("failed to describe %q: %w", table, err)
	}
	colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing to profile
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var names []string
	for _, cr := range colsRaw {
		name := ""
		for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
			if v, ok := cr[k].(string); ok && v != "" {
				name = v
				break
			}
		}
		if name == "" || !isPlainIdentifier(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	fmt.Fprintf(&b, "Profile of %s:\n", table)
	for _, col := range names {
		q := fmt.Sprintf(
			`SELECT COUNT(*), SUM(CASE WHEN %s IS NULL THEN 1 ELSE 0 END), COUNT(DISTINCT %s), MIN(%s), MAX(%s) FROM %s`,
			quoteIdent(col), quoteIdent(col), quoteIdent(col), quoteIdent(col), quoteIdent(table))
		rows, qerr := db.Query(ctx, q)
		if qerr != nil {
			fmt.Fprintf(&b, "- %s: unavailable (%v)\n", col, qerr)
			continue
		}
		if rows.Next() {
			var total, nulls, distinct int64
			var minV, maxV interface{}
			if scanErr := rows.Scan(&total, &nulls, &distinct, &minV, &maxV); scanErr == nil {
				line := fmt.Sprintf("- %s: rows: %d, distinct: %d, nulls: %d",
					col, total, distinct, nulls)
				if minV != nil && maxV != nil {
					line += fmt.Sprintf(", range: [%v .. %v]", renderScalar(minV), renderScalar(maxV))
				}
				b.WriteString(line + "\n")
			}
		}
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing profile rows for %s.%s: %v", table, col, cerr)
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
