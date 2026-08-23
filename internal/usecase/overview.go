package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// Database overview: one composed call answering "what am I working
// with?" — engine, shape counts, row totals, and sensitive-column
// suspects. The onboarding snapshot before any targeted query.

// DatabaseOverview renders the database's shape in one pass: table and
// column counts, index count, FK edge count, exact row total (SQLite)
// or catalog estimate, plus columns whose names trip the PII heuristics.
func (uc *DatabaseUseCase) DatabaseOverview(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}

	var tables []string
	for _, tr := range tablesRaw {
		name := metaString(tr, "table_name")
		if name == "" {
			name = metaString(tr, "name")
		}
		if name == "" || strings.HasPrefix(name, "sqlite_") {
			continue
		}
		tables = append(tables, name)
	}
	sort.Strings(tables)

	cols, indexes, fkEdges, sensitive := 0, 0, 0, 0
	var sensNames []string
	sensSeen := map[string]bool{}
	rowsTotal := int64(0)
	rowsExact := true

	db, err := uc.repo.GetDatabase(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}
	for _, t := range tables {
		desc, derr := uc.DescribeTable(ctx, dbID, t)
		if derr != nil {
			continue // unreadable table: skip from shape counts
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent means empty table
		cols += len(colsRaw)
		for _, cr := range colsRaw {
			colName := ""
			for _, k := range []string{"name", "column_name", "COLUMN_NAME"} {
				if v, ok2 := cr[k].(string); ok2 && v != "" {
					colName = v
					break
				}
			}
			norm := normalizeColumnName(colName)
			for _, p := range sensitivePatterns {
				if p.fragment != "" && strings.Contains(norm, p.fragment) {
					sensitive++
					key := t + "." + colName
					if !sensSeen[key] {
						sensSeen[key] = true
						sensNames = append(sensNames, key)
					}
					break
				}
			}
		}
		idxRaw, _ := desc["indexes"].([]map[string]interface{}) //nolint:errcheck // absent means none
		indexes += len(idxRaw)
		conRaw, _ := describeConstraintRows(desc["constraints"])
		for _, c := range conRaw {
			if metaString(c, "constraint_type") == "FOREIGN KEY" {
				fkEdges++
			}
		}

		if rows, qerr := db.Query(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(t))); qerr == nil {
			if rows.Next() {
				var n int64
				if scanErr := rows.Scan(&n); scanErr == nil {
					rowsTotal += n
				}
			}
			if cerr := rows.Close(); cerr != nil {
				logger.Error("error closing overview rows for %s: %v", t, cerr)
			}
		} else {
			rowsExact = false // estimate path not implemented per-engine; note it
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Overview of %q (engine: %s):\n", dbID, dbType)
	fmt.Fprintf(&b, "- shape: %d table(s), %d column(s), %d index(es)\n", len(tables), cols, indexes)
	fmt.Fprintf(&b, "- relationships: %d foreign-key edge(s)\n", fkEdges)
	label := "row(s) counted"
	if !rowsExact {
		label += " (some tables unreadable — total partial)"
	}
	fmt.Fprintf(&b, "- rows: %d %s\n", rowsTotal, label)
	if sensitive > 0 {
		sort.Strings(sensNames)
		fmt.Fprintf(&b, "- potential PII columns (%d): %s\n", sensitive, strings.Join(sensNames, ", "))
	} else {
		b.WriteString("- potential PII columns: none by name heuristic (run schema format=sensitive for content patterns)\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
