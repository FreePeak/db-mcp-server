package usecase

import (
	"context"

	"fmt"
	"github.com/FreePeak/db-mcp-server/internal/domain"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Value search: locate a literal across every textual column of every
// table — "which table holds this email/UUID?" before any query is
// written. Read-only LIKE probes with escaped wildcards; per-table
// failure degrades to a note, never fails the search.

type valueHit struct {
	table  string
	column string
	count  int64
}

func (uc *DatabaseUseCase) SearchValues(ctx context.Context, dbID, needle string) (string, error) {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return "", fmt.Errorf("search value must not be empty")
	}
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}
	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	pattern := "%" + escapeLike(needle) + "%"
	var hits []valueHit
	var skipped []string

	for _, tr := range tablesRaw {
		tableName := ""
		for _, k := range []string{"name", "table_name", "tableName", "TABLE_NAME"} {
			if v, ok := tr[k].(string); ok && v != "" {
				tableName = v
				break
			}
		}
		if strings.TrimSpace(tableName) == "" || strings.HasPrefix(tableName, "sqlite_") {
			continue
		}
		desc, err := uc.DescribeTable(ctx, dbID, tableName)
		if err != nil {
			skipped = append(skipped, tableName)
			continue
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing searchable
		var textCols []string
		for _, cr := range colsRaw {
			colName := ""
			colType := ""
			for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
				if v, ok := cr[k].(string); ok && v != "" {
					colName = v
					break
				}
			}
			for _, k := range []string{"type", "data_type", "Type", "DATA_TYPE"} {
				if v, ok := cr[k].(string); ok && v != "" {
					colType = v
					break
				}
			}
			if colName != "" && isTextualType(colType) {
				textCols = append(textCols, colName)
			}
		}
		if len(textCols) == 0 {
			continue
		}

		conds := make([]string, len(textCols))
		args := make([]interface{}, len(textCols))
		for i, c := range textCols {
			conds[i] = fmt.Sprintf("%s LIKE ? ESCAPE '\\'", quoteIdent(c))
			args[i] = pattern
		}
		q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s",
			quoteIdent(tableName), strings.Join(conds, " OR "))
		rows, err := db.Query(ctx, q, args...)
		if err != nil {
			skipped = append(skipped, tableName)
			continue
		}
		var n int64
		if rows.Next() {
			if scanErr := rows.Scan(&n); scanErr != nil {
				n = 0
			}
		}
		// Close before issuing the per-column probes: SQLite-style single-
		// connection pools cannot run a nested query while these rows are
		// open.
		if cerr := rows.Close(); cerr != nil {
			logger.Error("error closing value-search rows: %v", cerr)
		}
		if n > 0 {
			for _, c := range textCols {
				cn, cerr := uc.countColumnMatches(ctx, db, tableName, c, pattern)
				if cerr == nil && cn > 0 {
					hits = append(hits, valueHit{tableName, c, cn})
				}
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].table != hits[j].table {
			return hits[i].table < hits[j].table
		}
		return hits[i].column < hits[j].column
	})

	if len(hits) == 0 {
		out := fmt.Sprintf("No matches for %q.", needle)
		if len(skipped) > 0 {
			out += " Unreadable tables skipped: " + strings.Join(skipped, ", ") + "."
		}
		return out, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Matches for %q:\n", needle)
	for _, h := range hits {
		fmt.Fprintf(&b, "%s.%s: %d row(s)\n", h.table, h.column, h.count)
	}
	if len(skipped) > 0 {
		b.WriteString("Unreadable tables skipped: " + strings.Join(skipped, ", ") + "\n")
	}
	return b.String(), nil
}

func (uc *DatabaseUseCase) countColumnMatches(ctx context.Context, db domain.Database, table, column, pattern string) (int64, error) {
	rows, err := db.Query(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s LIKE ? ESCAPE '\\'",
			quoteIdent(table), quoteIdent(column)), pattern)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logger.Error("error closing match-count rows: %v", closeErr)
		}
	}()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var n int64
	err = rows.Scan(&n)
	return n, err
}

// escapeLike neutralizes LIKE wildcards in the needle so literal values
// match literally.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}
