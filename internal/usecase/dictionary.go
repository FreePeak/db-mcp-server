package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Data dictionary: the whole schema rendered as Markdown — one section
// per table with column/type/PK/FK lines — ready to paste into a repo's
// docs instead of hand-writing it from repeated describes.

// DataDictionary renders every user table as a Markdown data dictionary.
func (uc *DatabaseUseCase) DataDictionary(ctx context.Context, dbID string) (string, error) {
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

	var b strings.Builder
	fmt.Fprintf(&b, "# Data dictionary: %s\n", dbID)
	for _, t := range tables {
		desc, derr := uc.DescribeTable(ctx, dbID, t)
		if derr != nil {
			fmt.Fprintf(&b, "\n## %s\n\n(describe failed: %v)\n", t, derr)
			continue
		}
		pkCols := map[string]bool{}
		fkTargets := map[string]string{} // column -> referenced_table.column
		if cons, _ := describeConstraintRows(desc["constraints"]); cons != nil {
			for _, c := range cons {
				col := metaString(c, "column_name")
				switch metaString(c, "constraint_type") {
				case "PRIMARY KEY":
					if col != "" {
						pkCols[col] = true
					}
				case "FOREIGN KEY":
					refTable := metaString(c, "referenced_table")
					refCol := metaString(c, "referenced_column")
					if col != "" && refTable != "" {
						fkTargets[col] = refTable + "." + refCol
					}
				}
			}
		}
		fmt.Fprintf(&b, "\n## %s\n\n| column | type | notes |\n|---|---|---|\n", t)
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns renders an empty table
		for _, cr := range colsRaw {
			colName := metaString(cr, "name")
			if colName == "" {
				colName = metaString(cr, "column_name")
			}
			if colName == "" {
				colName = metaString(cr, "COLUMN_NAME")
			}
			colType := metaString(cr, "type")
			if colType == "" {
				colType = metaString(cr, "data_type")
			}
			if colType == "" {
				colType = metaString(cr, "Type")
			}
			var notes []string
			if pkCols[colName] {
				notes = append(notes, "PK")
			}
			if tgt, isFK := fkTargets[colName]; isFK {
				notes = append(notes, "FK -> "+tgt)
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", colName, colType, strings.Join(notes, ", "))
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
