package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Column-type consistency: the same column name appearing with
// divergent types across tables (customer_id INTEGER here, TEXT there)
// signals a bad migration, a broken FK intent, or copy-paste drift.
// Joins on such columns coerce or fail at runtime; this audit is static
// over the DescribeTable metadata already collected.

// FindTypeInconsistencies renders every column name that appears in two
// or more user tables with more than one distinct type, listing each
// table's declared type. Identical types differing only in case are not
// divergence.
func (uc *DatabaseUseCase) FindTypeInconsistencies(ctx context.Context, dbID string) (string, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}

	scannedTables := 0
	type occurrence struct{ table, typ string }
	byColumn := map[string][]occurrence{}
	for _, tr := range tablesRaw {
		tableName := metaString(tr, "table_name")
		if tableName == "" {
			tableName = metaString(tr, "name")
		}
		if tableName == "" || strings.HasPrefix(tableName, "sqlite_") || !isPlainIdentifier(tableName) {
			continue
		}
		desc, derr := uc.DescribeTable(ctx, dbID, tableName)
		if derr != nil {
			continue // unreadable table: skip rather than fail the audit
		}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means nothing to compare
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
			if colName == "" {
				continue
			}
			byColumn[colName] = append(byColumn[colName], occurrence{tableName, strings.ToLower(strings.TrimSpace(colType))})
		}
		scannedTables++
	}

	var names []string
	for col, occs := range byColumn {
		types := map[string]bool{}
		for _, o := range occs {
			types[o.typ] = true
		}
		if len(occs) >= 2 && len(types) > 1 {
			names = append(names, col)
		}
	}
	sort.Strings(names)

	if scannedTables < 2 {
		return "No column appears in two or more tables: nothing to cross-check.", nil
	}
	if len(names) == 0 {
		return fmt.Sprintf("Type-consistent: every shared column name across %d table(s) has one type.", scannedTables), nil
	}

	sort.Slice(byColumn[names[0]], func(i, j int) bool { return byColumn[names[0]][i].table < byColumn[names[0]][j].table })
	var b strings.Builder
	fmt.Fprintf(&b, "%d shared column(s) have inconsistent types across %d table(s):\n", len(names), scannedTables)
	for _, col := range names {
		occs := byColumn[col]
		sort.Slice(occs, func(i, j int) bool { return occs[i].table < occs[j].table })
		parts := make([]string, len(occs))
		for i, o := range occs {
			parts[i] = fmt.Sprintf("%s.%s=%s", o.table, col, o.typ)
		}
		fmt.Fprintf(&b, "- %s: %s — joins on this column will coerce or fail\n",
			col, strings.Join(parts, ", "))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
