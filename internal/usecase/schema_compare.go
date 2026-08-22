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

// schemaSnapshot is one database's tables mapped to column-name → type,
// plus per-table index fingerprints (name → normalized definition).
type schemaSnapshot struct {
	columns map[string]map[string]string
	indexes map[string]map[string]string
}

func newSchemaSnapshot() *schemaSnapshot {
	return &schemaSnapshot{
		columns: map[string]map[string]string{},
		indexes: map[string]map[string]string{},
	}
}

// normalizeDefinition collapses whitespace and lowercases an index DDL so
// cosmetic formatting differences do not read as drift.
func normalizeDefinition(def string) string {
	return strings.Join(strings.Fields(strings.ToLower(def)), " ")
}

// collectSchemaSnapshot walks GetDatabaseInfo + DescribeTable for dbID.
func (uc *DatabaseUseCase) collectSchemaSnapshot(ctx context.Context, dbID string) (schemaSnapshot, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return schemaSnapshot{}, fmt.Errorf("failed to list tables on %q: %w", dbID, err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return schemaSnapshot{}, fmt.Errorf("no table listing available for %q", dbID)
	}
	snap := newSchemaSnapshot()
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
		snap.columns[tableName] = cols

		idxRaw, _ := desc["indexes"].([]map[string]interface{}) //nolint:errcheck // absent indexes means none
		if len(idxRaw) > 0 && snap.indexes[tableName] == nil {
			snap.indexes[tableName] = map[string]string{}
		}
		for _, ir := range idxRaw {
			name, _ := ir["index_name"].(string) //nolint:errcheck // absent name means unidentifiable
			def, _ := ir["definition"].(string)  //nolint:errcheck // absent definition means unidentifiable
			if name == "" || def == "" {
				continue
			}
			snap.indexes[tableName][name] = normalizeDefinition(def)
		}
	}
	return *snap, nil
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
	for t := range snapA.columns {
		tables[t] = true
	}
	for t := range snapB.columns {
		tables[t] = true
	}
	names := make([]string, 0, len(tables))
	for t := range tables {
		names = append(names, t)
	}
	sort.Strings(names)

	for _, t := range names {
		colsA, inA := snapA.columns[t]
		colsB, inB := snapB.columns[t]
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

		idxNames := map[string]bool{}
		for i := range snapA.indexes[t] {
			idxNames[i] = true
		}
		for i := range snapB.indexes[t] {
			idxNames[i] = true
		}
		sortedIdx := make([]string, 0, len(idxNames))
		for i := range idxNames {
			sortedIdx = append(sortedIdx, i)
		}
		sort.Strings(sortedIdx)
		for _, i := range sortedIdx {
			fa, okA := snapA.indexes[t][i]
			fb, okB := snapB.indexes[t][i]
			switch {
			case !okB:
				lines = append(lines, fmt.Sprintf("table %q index %q: only in %s", t, i, dbIDA))
			case !okA:
				lines = append(lines, fmt.Sprintf("table %q index %q: only in %s", t, i, dbIDB))
			case fa != fb:
				lines = append(lines, fmt.Sprintf("table %q index %q: definition differs (%s=%s, %s=%s)", t, i, dbIDA, fa, dbIDB, fb))
			}
		}
	}

	if len(lines) == 0 {
		return fmt.Sprintf("Schemas match: %d common table(s), no differences.", len(snapA.columns)), nil
	}
	sort.Strings(lines)
	return "Schema differences between " + dbIDA + " and " + dbIDB + ":\n- " + strings.Join(lines, "\n- "), nil
}
