package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Cross-database schema compare: diff two databases' table/column shapes
// (e.g. staging vs production) so an agent can verify a migration landed
// on both sides. Structural only — no row data is read.

// schemaSnapshot is one database's tables mapped to column-name → type.
type schemaSnapshot map[string]map[string]string

// collectSchemaSnapshot walks GetDatabaseInfo + DescribeTable for dbID.
func (uc *DatabaseUseCase) collectSchemaSnapshot(ctx context.Context, dbID string) (schemaSnapshot, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables on %q: %w", dbID, err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no table listing available for %q", dbID)
	}
	snap := schemaSnapshot{}
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
			continue // unreadable table: skip rather than fail the compare
		}
		cols := map[string]string{}
		colsRaw, _ := desc["columns"].([]map[string]interface{}) //nolint:errcheck // absent columns means empty table
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
			cols[colName] = strings.ToLower(colType)
		}
		snap[tableName] = cols
	}
	return snap, nil
}

// CompareSchemas renders the structural differences between two databases:
// tables present on only one side, and per shared table the columns missing
// or type-divergent on either side.
func (uc *DatabaseUseCase) CompareSchemas(ctx context.Context, dbIDA, dbIDB string) (string, error) {
	snapA, err := uc.collectSchemaSnapshot(ctx, dbIDA)
	if err != nil {
		return "", err
	}
	snapB, err := uc.collectSchemaSnapshot(ctx, dbIDB)
	if err != nil {
		return "", err
	}

	var lines []string
	tables := map[string]bool{}
	for t := range snapA {
		tables[t] = true
	}
	for t := range snapB {
		tables[t] = true
	}
	names := make([]string, 0, len(tables))
	for t := range tables {
		names = append(names, t)
	}
	sort.Strings(names)

	for _, t := range names {
		colsA, inA := snapA[t]
		colsB, inB := snapB[t]
		switch {
		case !inB:
			lines = append(lines, fmt.Sprintf("table %q: only in %s", t, dbIDA))
			continue
		case !inA:
			lines = append(lines, fmt.Sprintf("table %q: only in %s", t, dbIDB))
			continue
		}
		colNames := map[string]bool{}
		for c := range colsA {
			colNames[c] = true
		}
		for c := range colsB {
			colNames[c] = true
		}
		sortedCols := make([]string, 0, len(colNames))
		for c := range colNames {
			sortedCols = append(sortedCols, c)
		}
		sort.Strings(sortedCols)
		for _, c := range sortedCols {
			ta, okA := colsA[c]
			tb, okB := colsB[c]
			switch {
			case !okA:
				lines = append(lines, fmt.Sprintf("table %q column %q: missing in %s (present as %s)", t, c, dbIDA, tb))
			case !okB:
				lines = append(lines, fmt.Sprintf("table %q column %q: missing in %s (present as %s)", t, c, dbIDB, ta))
			case ta != tb:
				lines = append(lines, fmt.Sprintf("table %q column %q: type differs (%s=%s, %s=%s)", t, c, dbIDA, ta, dbIDB, tb))
			}
		}
	}

	if len(lines) == 0 {
		return fmt.Sprintf("Schemas match: %d common table(s), no differences.", len(snapA)), nil
	}
	sort.Strings(lines)
	return "Schema differences between " + dbIDA + " and " + dbIDB + ":\n- " + strings.Join(lines, "\n- "), nil
}
