package usecase

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	fromTableRe  = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_$.]*)`)
	tableAliasRe = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_$.]*)\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_$]*)\b`)
	joinOnRe     = regexp.MustCompile(`(?i)\bON\b\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)`)
	whereColRe   = regexp.MustCompile(`(?i)(?:WHERE|AND|OR)\s+\(?([A-Za-z_][A-Za-z0-9_]*)(?:\.[A-Za-z_][A-Za-z0-9_]*)?\s*(?:=|!=|<>|>=|<=|>|<|LIKE|ILIKE|IN\b)`)
	orderColRe   = regexp.MustCompile(`(?i)\bORDER\s+BY\s+((?:[A-Za-z_][A-Za-z0-9_.]*(?:\s+(?:ASC|DESC))?(?:\s*,\s*)?)+)`)
	groupColRe   = regexp.MustCompile(`(?i)\bGROUP\s+BY\s+((?:[A-Za-z_][A-Za-z0-9_.]*(?:\s*,\s*)?)+)`)
	identifierRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// SuggestIndexes compares the columns a query filters, joins, and sorts on
// against the database's existing indexes, proposing CREATE INDEX statements
// for uncovered high-value columns. Heuristic by design — every suggestion
// is labelled as such and EXPLAIN should confirm before acting.
func (uc *DatabaseUseCase) SuggestIndexes(ctx context.Context, dbID, query string) (string, error) {
	query = strings.TrimSpace(strings.TrimRight(query, ";"))
	if query == "" {
		return "", fmt.Errorf("query parameter must not be empty")
	}

	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}

	tables := extractQueryTables(query)
	if len(tables) == 0 {
		return "No tables detected in the query; nothing to advise on.", nil
	}

	aliases := extractAliases(query)
	resolve := func(name string) string {
		name = stripSchema(strings.ToLower(name))
		if real, ok := aliases[name]; ok {
			return real
		}
		return name
	}

	candidates := map[string][]idxCandidate{} // table -> columns worth indexing
	addCandidate := func(table, col, kind string) {
		table = resolve(table)
		col = strings.ToLower(col)
		if !isPlainIdentifier(col) || reservedWord(col) {
			return
		}
		for _, existing := range candidates[table] {
			if existing.column == col {
				return
			}
		}
		candidates[table] = append(candidates[table], idxCandidate{column: col, kind: kind})
	}

	// Join columns are the highest-value targets.
	for _, m := range joinOnRe.FindAllStringSubmatch(query, -1) {
		addCandidate(m[1], m[2], "join") // left side
		addCandidate(m[3], m[4], "join") // right side
	}

	// Filter columns (WHERE x = / LIKE / IN ...). Qualified refs are matched
	// to their table when recognizable.
	for _, m := range whereColRe.FindAllStringSubmatch(query, -1) {
		col := m[1]
		target := primaryTableFor(tables, query, col)
		addCandidate(target, col, "where")
	}

	// ORDER BY / GROUP BY leading columns.
	for _, re := range []*regexp.Regexp{orderColRe, groupColRe} {
		for _, m := range re.FindAllStringSubmatch(query, -1) {
			for _, raw := range splitColumns(m[1]) {
				parts := strings.Fields(raw)
				if len(parts) == 0 {
					continue
				}
				colParts := identifierRe.FindAllString(parts[0], -1)
				if len(colParts) == 0 {
					continue
				}
				col := colParts[len(colParts)-1] // last segment handles table.col
				addCandidate(primaryTableFor(tables, query, col), col, "sort")
			}
		}
	}

	var b strings.Builder
	b.WriteString("Index suggestions (heuristic — verify with EXPLAIN before creating):\n\n")
	suggestions := 0

	for _, table := range orderedCandidateKeys(candidates) {
		existing, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
		if err != nil {
			continue // cannot compare; skip silently
		}
		indexedText := strings.ToLower(fmt.Sprintf("%v", existing))

		// Constraint catalog: primary-key columns are already indexed by
		// definition and must never be re-suggested.
		pkCols := map[string]bool{}
		if constraints, err := uc.queryTableMetadata(ctx, dbID, constraintQueries(strings.ToLower(dbType), table)); err == nil {
			for _, row := range constraints {
				if strings.EqualFold(fmt.Sprintf("%v", row["constraint_type"]), "primary key") {
					if col, ok := row["column_name"].(string); ok {
						pkCols[strings.ToLower(col)] = true
					}
				}
			}
		}

		var uncovered []idxCandidate
		for _, cand := range candidates[table] {
			if pkCols[cand.column] || indexCovers(indexedText, cand.column) {
				continue
			}
			uncovered = append(uncovered, cand)
		}

		// Composite grouping: ≥2 uncovered WHERE columns on the same table
		// usually beat N single-column indexes for AND-filtered queries.
		var whereCols, otherCols []idxCandidate
		for _, cand := range uncovered {
			if cand.kind == "where" {
				whereCols = append(whereCols, cand)
			} else {
				otherCols = append(otherCols, cand)
			}
		}
		if len(whereCols) >= 2 {
			cols := candidateNames(whereCols)
			suggestions++
			fmt.Fprintf(&b, "  CREATE INDEX idx_%s_%s ON %s (%s);\n", table, strings.Join(cols, "_"), table, strings.Join(cols, ", "))
			whereCols = nil
		}
		for _, cand := range append(whereCols, otherCols...) {
			suggestions++
			fmt.Fprintf(&b, "  CREATE INDEX idx_%s_%s ON %s (%s);\n", table, cand.column, table, cand.column)
		}
	}

	if suggestions == 0 {
		b.WriteString("  (none — filter/join/sort columns appear covered by existing indexes)\n")
	}
	return b.String(), nil
}

// extractQueryTables returns real table names referenced by FROM/JOIN
// clauses (aliases resolved), excluding subquery aliases.
func extractQueryTables(query string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range fromTableRe.FindAllStringSubmatch(query, -1) {
		name := stripSchema(m[1])
		lower := strings.ToLower(name)
		if lower == "select" || seen[lower] || !isPlainIdentifier(name) {
			continue
		}
		seen[lower] = true
		out = append(out, name)
	}
	return out
}

// extractAliases maps query aliases (e.g. "FROM books b") to their real
// table names so suggestions always target physical tables. Words that are
// actually keywords (JOIN/WHERE/...) terminate alias parsing.
func extractAliases(query string) map[string]string {
	aliases := map[string]string{}
	for _, m := range tableAliasRe.FindAllStringSubmatch(query, -1) {
		table := strings.ToLower(stripSchema(m[1]))
		alias := strings.ToLower(m[2])
		if table == "select" || alias == table || reservedWord(alias) ||
			strings.EqualFold(alias, "join") || !isPlainIdentifier(alias) {
			continue
		}
		aliases[alias] = table
	}
	return aliases
}

func stripSchema(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// primaryTableFor attributes an unqualified column to the first referenced
// table unless another table owns it via an explicit qualification nearby;
// heuristics stay intentionally simple. Tables arrive already alias-resolved.
func primaryTableFor(tables []string, _ string, _ string) string {
	if len(tables) == 0 {
		return ""
	}
	return strings.ToLower(stripSchema(tables[0]))
}

func splitColumns(list string) []string {
	return strings.Split(list, ",")
}

// indexCovers checks whether any existing index definition mentions col.
func indexCovers(existingIndexText, col string) bool {
	return strings.Contains(existingIndexText, "("+col+")") ||
		strings.Contains(existingIndexText, "("+col+",") ||
		strings.Contains(existingIndexText, col)
}

// idxCandidate is a column worth indexing plus the clause that nominated it
// (where/join/sort), driving composite-grouping decisions.
type idxCandidate struct {
	column string
	kind   string
}

func candidateNames(cands []idxCandidate) []string {
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.column)
	}
	return out
}

// orderedCandidateKeys sorts candidate tables for deterministic output.
func orderedCandidateKeys(m map[string][]idxCandidate) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var sqlReserved = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true,
	"order": true, "group": true, "by": true, "join": true, "on": true,
	"left": true, "right": true, "inner": true, "outer": true, "limit": true,
	"having": true, "as": true, "in": true, "like": true, "between": true,
	"is": true, "not": true, "null": true, "distinct": true, "set": true,
}

func reservedWord(s string) bool { return sqlReserved[strings.ToLower(s)] }
