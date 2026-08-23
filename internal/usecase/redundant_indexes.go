package usecase

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Redundant-index detection: an index whose column list is a prefix of
// another index's columns serves only queries the wider index already
// covers — pure write amplification and disk waste. Unique indexes are
// never redundant even when covered (they enforce a constraint).

var indexColsRe = regexp.MustCompile(`\(([^()]*)\)\s*$`)
var indexUniqueRe = regexp.MustCompile(`(?i)\bUNIQUE\b`)

// indexColumns extracts the trailing parenthesized column list of a
// CREATE INDEX definition as ordered lowercase names; ok=false when the
// shape is unrecognizable.
func indexColumns(definition string) ([]string, bool) {
	m := indexColsRe.FindStringSubmatch(strings.TrimSpace(definition))
	if len(m) < 2 {
		return nil, false
	}
	var cols []string
	for _, part := range strings.Split(m[1], ",") {
		f := strings.Fields(strings.TrimSpace(part))
		if len(f) == 0 {
			continue
		}
		cols = append(cols, strings.ToLower(strings.Trim(f[0], `"`+"`")))
	}
	if len(cols) == 0 {
		return nil, false
	}
	return cols, true
}

// coversPrefix reports whether wide's column list starts with narrow's.
func coversPrefix(narrow, wide []string) bool {
	if len(narrow) >= len(wide) {
		return false
	}
	for i := range narrow {
		if narrow[i] != wide[i] {
			return false
		}
	}
	return true
}

// sameColumns reports whether two column lists are element-wise equal.
func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FindRedundantIndexes renders every non-unique index whose columns are
// a prefix of a wider sibling on the same table, plus exact duplicate
// pairs (identical column lists reported once, later index as the drop
// candidate).
func (uc *DatabaseUseCase) FindRedundantIndexes(ctx context.Context, dbID string) (string, error) {
	info, err := uc.GetDatabaseInfo(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to list tables: %w", err)
	}
	tablesRaw, ok := info["tables"].([]map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no table listing available for %q", dbID)
	}

	type pair struct {
		narrow, wide, table string
		duplicate           bool // identical column lists vs prefix-covered
	}
	var redundant []pair
	scanned := 0
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
		idxRaw, _ := desc["indexes"].([]map[string]interface{}) //nolint:errcheck // absent indexes means none
		type idx struct {
			name   string
			cols   []string
			unique bool
		}
		var idxs []idx
		for _, ir := range idxRaw {
			name := metaString(ir, "index_name")
			def := metaString(ir, "definition")
			if name == "" || def == "" {
				continue
			}
			cols, ok := indexColumns(def)
			if !ok {
				continue
			}
			idxs = append(idxs, idx{name: name, cols: cols, unique: indexUniqueRe.MatchString(def)})
		}
		scanned += len(idxs)
		for i, a := range idxs {
			if a.unique {
				continue
			}
			for j, b := range idxs {
				if i == j {
					continue
				}
				if !b.unique && coversPrefix(a.cols, b.cols) {
					redundant = append(redundant, pair{
						narrow: a.name,
						wide:   b.name,
						table:  tableName,
					})
					break // first covering index is enough evidence
				}
				if !b.unique && j > i && sameColumns(a.cols, b.cols) {
					// Exact duplicate: report once (j > i) with the later
					// index as the drop candidate.
					redundant = append(redundant, pair{
						narrow:    b.name,
						wide:      a.name,
						table:     tableName,
						duplicate: true,
					})
					break // reported once per duplicate pair
				}
			}
		}
	}
	sort.Slice(redundant, func(i, j int) bool {
		if redundant[i].table != redundant[j].table {
			return redundant[i].table < redundant[j].table
		}
		return redundant[i].narrow < redundant[j].narrow
	})

	if len(redundant) == 0 {
		return fmt.Sprintf("No redundant indexes across %d scanned index(es): no covered prefixes or duplicates.", scanned), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d redundant index(es) across %d scanned:\n", len(redundant), scanned)
	for _, p := range redundant {
		relation := "covered by"
		if p.duplicate {
			relation = "exact duplicate of"
		}
		fmt.Fprintf(&b, "- %s.%s: %s %s — candidate for DROP INDEX %s\n",
			p.table, p.narrow, relation, p.wide, p.narrow)
	}
	b.WriteString("Verify with real query patterns before dropping.\n")
	return strings.TrimRight(b.String(), "\n"), nil
}
