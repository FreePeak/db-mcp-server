package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// No-primary-key detection: a table without a PRIMARY KEY breaks
// row-based and logical replication (engines need an identity column),
// makes individual rows unaddressable for targeted updates, and hides
// duplicate rows. Static audit over the constraint catalog DescribeTable
// already returns.

// FindTablesWithoutPK renders every user table that has no PRIMARY KEY
// constraint, with the concrete risks named.
func (uc *DatabaseUseCase) FindTablesWithoutPK(ctx context.Context, dbID string) (string, error) {
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
	if len(tables) == 0 {
		return "No user tables to audit.", nil
	}

	var keyless []string
	for _, t := range tables {
		desc, derr := uc.DescribeTable(ctx, dbID, t)
		if derr != nil {
			continue // unreadable table: skip rather than fail the audit
		}
		conRaw, _ := describeConstraintRows(desc["constraints"])
		hasPK := false
		for _, c := range conRaw {
			if metaString(c, "constraint_type") == "PRIMARY KEY" {
				hasPK = true
				break
			}
		}
		if !hasPK {
			keyless = append(keyless, t)
		}
	}

	if len(keyless) == 0 {
		return fmt.Sprintf("All %d user table(s) have a primary key.", len(tables)), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d table(s) lack a PRIMARY KEY:\n", len(keyless), len(tables))
	for _, t := range keyless {
		fmt.Fprintf(&b, "- %s: rows are unaddressable (updates/deletes need full-row predicates), "+
			"duplicates go undetected, and row-based/logical replication may break\n", t)
	}
	b.WriteString("Consider adding a surrogate key unless the table is intentionally keyless (e.g. pure log append).")
	return strings.TrimRight(b.String(), "\n"), nil
}
