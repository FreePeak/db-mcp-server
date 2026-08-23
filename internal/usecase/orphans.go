package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/logger"
)

// FK integrity audit: count orphaned child rows per foreign-key edge —
// children whose fk column has no matching parent. Catches corruption
// from disabled constraints, legacy imports, and partial deletes.

// AuditOrphans walks every FOREIGN KEY edge in the database and counts
// child rows whose referencing column has no parent match.
func (uc *DatabaseUseCase) AuditOrphans(ctx context.Context, dbID string) (string, error) {
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

	type violation struct {
		childTable  string
		childCol    string
		parentTable string
		parentCol   string
		count       int64
	}
	var violations []violation
	var edges []string
	skipped := 0

	for _, tr := range tablesRaw {
		tableName := metaString(tr, "table_name")
		if tableName == "" {
			tableName = metaString(tr, "name")
		}
		if tableName == "" || !isPlainIdentifier(tableName) || strings.HasPrefix(tableName, "sqlite_") {
			continue
		}
		desc, derr := uc.DescribeTable(ctx, dbID, tableName)
		if derr != nil {
			skipped++
			continue
		}
		constraints, _ := describeConstraintRows(desc["constraints"])
		for _, c := range constraints {
			if metaString(c, "constraint_type") != "FOREIGN KEY" {
				continue
			}
			col := metaString(c, "column_name")
			refTable := metaString(c, "referenced_table")
			refCol := metaString(c, "referenced_column")
			if col == "" || refTable == "" || refCol == "" ||
				!isPlainIdentifier(col) || !isPlainIdentifier(refTable) || !isPlainIdentifier(refCol) {
				continue // unidentifiable edge: skip rather than guess
			}
			q := fmt.Sprintf("SELECT COUNT(*) FROM %s c LEFT JOIN %s p ON c.%s = p.%s WHERE c.%s IS NOT NULL AND p.%s IS NULL",
				quoteIdent(tableName), quoteIdent(refTable),
				quoteIdent(col), quoteIdent(refCol), quoteIdent(col), quoteIdent(refCol))
			rows, qerr := db.Query(ctx, q)
			if qerr != nil {
				skipped++
				continue
			}
			var n int64
			if rows.Next() {
				_ = rows.Scan(&n) //nolint:errcheck // COUNT(*) always scans
			}
			if cerr := rows.Close(); cerr != nil {
				logger.Error("error closing orphan-count rows: %v", cerr)
			}
			edge := fmt.Sprintf("%s.%s -> %s.%s", tableName, col, refTable, refCol)
			edges = append(edges, edge)
			if n > 0 {
				violations = append(violations, violation{tableName, col, refTable, refCol, n})
			}
		}
	}

	sort.Slice(violations, func(i, j int) bool { return violations[i].count > violations[j].count })
	if len(violations) == 0 {
		out := fmt.Sprintf("No violations: %d foreign-key edge(s) checked, all child rows have parents.", len(edges))
		if skipped > 0 {
			out += fmt.Sprintf(" (%d table(s) unreadable and skipped)", skipped)
		}
		return out, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d foreign-key edge(s) have orphaned rows:\n", len(violations), len(edges))
	for _, v := range violations {
		fmt.Fprintf(&b, "- %s.%s -> %s.%s: %d orphaned row(s)\n",
			v.childTable, v.childCol, v.parentTable, v.parentCol, v.count)
	}
	if skipped > 0 {
		fmt.Fprintf(&b, "(%d table(s) unreadable and skipped)\n", skipped)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
