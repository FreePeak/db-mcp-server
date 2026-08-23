package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Missing-FK-index detection: every unindexed foreign-key child column
// turns parent DELETE/UPDATE into a full scan of the child table (the
// engine must check referential integrity row by row). suggest_indexes
// needs a workload; this audit is static — FK edges vs index leading
// columns — and catches the problem before any slow query is run.

// fkEdge is one child column referencing a parent.
type fkEdge struct {
	childTable  string
	childColumn string
	parentTable string
	parentCol   string
}

// collectFKEdges walks every user table's constraints for FOREIGN KEY rows.
func (uc *DatabaseUseCase) collectFKEdges(ctx context.Context, dbID string, tables []string) []fkEdge {
	var edges []fkEdge
	for _, t := range tables {
		desc, err := uc.DescribeTable(ctx, dbID, t)
		if err != nil {
			continue // unreadable table: skip rather than fail the audit
		}
		conRaw, _ := describeConstraintRows(desc["constraints"])
		for _, c := range conRaw {
			if metaString(c, "constraint_type") != "FOREIGN KEY" {
				continue
			}
			col := metaString(c, "column_name")
			refTable := metaString(c, "referenced_table")
			if col == "" || !isPlainIdentifier(col) {
				continue
			}
			edges = append(edges, fkEdge{
				childTable:  t,
				childColumn: col,
				parentTable: refTable,
				parentCol:   metaString(c, "referenced_column"),
			})
		}
	}
	return edges
}

// tableLeadingIndexColumns maps each user table to the set of columns
// that lead at least one of its indexes.
func (uc *DatabaseUseCase) tableLeadingIndexColumns(ctx context.Context, dbID string, tables []string) map[string]map[string]bool {
	leading := map[string]map[string]bool{}
	for _, t := range tables {
		desc, err := uc.DescribeTable(ctx, dbID, t)
		if err != nil {
			continue
		}
		idxRaw, _ := desc["indexes"].([]map[string]interface{}) //nolint:errcheck // absent indexes means none
		for _, ir := range idxRaw {
			def := metaString(ir, "definition")
			cols, ok := indexColumns(def)
			if !ok || len(cols) == 0 {
				continue
			}
			if leading[t] == nil {
				leading[t] = map[string]bool{}
			}
			leading[t][cols[0]] = true
		}
	}
	return leading
}

// FindMissingFKIndexes renders every foreign-key child column that no
// index leads on, with candidate CREATE INDEX DDL.
func (uc *DatabaseUseCase) FindMissingFKIndexes(ctx context.Context, dbID string) (string, error) {
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
		if name == "" || strings.HasPrefix(name, "sqlite_") || !isPlainIdentifier(name) {
			continue
		}
		tables = append(tables, name)
	}
	sort.Strings(tables)

	edges := uc.collectFKEdges(ctx, dbID, tables)
	if len(edges) == 0 {
		return fmt.Sprintf("No foreign keys across %d scanned table(s): nothing to audit.", len(tables)), nil
	}
	leading := uc.tableLeadingIndexColumns(ctx, dbID, tables)

	var missing []fkEdge
	for _, e := range edges {
		if !leading[e.childTable][e.childColumn] {
			missing = append(missing, e)
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].childTable != missing[j].childTable {
			return missing[i].childTable < missing[j].childTable
		}
		return missing[i].childColumn < missing[j].childColumn
	})

	if len(missing) == 0 {
		return fmt.Sprintf("No missing foreign-key indexes: %d edge(s), all child columns indexed.", len(edges)), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d foreign-key column(s) lack a leading index:\n", len(missing), len(edges))
	for _, e := range missing {
		idxName := fmt.Sprintf("idx_%s_%s", e.childTable, e.childColumn)
		fmt.Fprintf(&b, "- %s.%s -> %s.%s: deletes on %s will scan %s — candidate: CREATE INDEX %s ON %s (%s)\n",
			e.childTable, e.childColumn, e.parentTable, e.parentCol,
			e.parentTable, e.childTable, idxName, e.childTable, e.childColumn)
	}
	b.WriteString("Verify write overhead is acceptable before creating.\n")
	return strings.TrimRight(b.String(), "\n"), nil
}
