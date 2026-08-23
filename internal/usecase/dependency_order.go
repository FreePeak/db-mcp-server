package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// FK dependency order: topologically sort tables so referenced tables
// come before referencing ones — the safe order for seeding, copying,
// or truncating without violating foreign keys. Circular references
// are flagged rather than silently mis-ordered.

// DependencyOrder renders the FK-safe ordering of every user table.
func (uc *DatabaseUseCase) DependencyOrder(ctx context.Context, dbID string) (string, error) {
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

	// edges[parent] lists children; parents must precede children.
	inDegree := map[string]int{}
	children := map[string][]string{}
	for _, t := range tables {
		inDegree[t] = 0
	}
	for _, t := range tables {
		desc, derr := uc.DescribeTable(ctx, dbID, t)
		if derr != nil {
			continue // unreadable table: keep it at the front (no known deps)
		}
		conRaw, _ := describeConstraintRows(desc["constraints"])
		for _, c := range conRaw {
			if metaString(c, "constraint_type") != "FOREIGN KEY" {
				continue
			}
			parent := metaString(c, "referenced_table")
			if parent == "" || parent == t || !isPlainIdentifier(parent) {
				continue
			}
			if _, known := inDegree[parent]; !known {
				continue // references a table outside this database's listing
			}
			children[parent] = append(children[parent], t)
			inDegree[t]++
		}
	}

	// Kahn's algorithm with deterministic tie-breaking.
	var ready []string
	for _, t := range tables {
		if inDegree[t] == 0 {
			ready = append(ready, t)
		}
	}
	var ordered []string
	for len(ready) > 0 {
		sort.Strings(ready)
		cur := ready[0]
		ready = ready[1:]
		ordered = append(ordered, cur)
		for _, ch := range children[cur] {
			inDegree[ch]--
			if inDegree[ch] == 0 {
				ready = append(ready, ch)
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "FK-safe order for %s (%d table(s); truncate in reverse):\n", dbID, len(ordered))
	for i, t := range ordered {
		fmt.Fprintf(&b, "%d. %s\n", i+1, t)
	}
	if len(ordered) < len(tables) {
		var stuck []string
		orderedSet := map[string]bool{}
		for _, t := range ordered {
			orderedSet[t] = true
		}
		for _, t := range tables {
			if !orderedSet[t] {
				stuck = append(stuck, t)
			}
		}
		sort.Strings(stuck)
		b.WriteString(fmt.Sprintf("Circular reference detected among: %s — break one FK temporarily to seed these.\n",
			strings.Join(stuck, ", ")))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
