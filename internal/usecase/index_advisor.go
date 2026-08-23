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
	joinOnRe     = regexp.MustCompile(`(?i)\bON\b\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)`)
	whereColRe   = regexp.MustCompile(`(?i)(?:WHERE|AND|OR)\s+\(?((?:[A-Za-z_][A-Za-z0-9_$]*)(?:\.[A-Za-z_][A-Za-z0-9_$]*)?)\s*(?:=|!=|<>|>=|<=|>|<|LIKE|ILIKE|IN\b)`)
	aliasRe      = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_$.]*)(?:\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_$]*))?`)
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
	aliases := extractAliasMap(query)

	candidates := map[string][]string{} // table -> columns worth indexing
	pushCandidate := func(table, col string) {
		table = stripSchema(table)
		if real, ok := aliases[strings.ToLower(table)]; ok {
			table = real // join aliases must resolve to actual tables
		}
		col = strings.ToLower(col)
		if !isPlainIdentifier(col) || reservedWord(col) {
			return
		}
		for _, existing := range candidates[table] {
			if existing == col {
				return
			}
		}
		candidates[table] = append(candidates[table], col)
	}

	// attributeRef resolves a possibly-qualified column reference
	// (b.author_id) to its owning table: explicit qualifiers go through the
	// alias map, bare columns fall back to the first referenced table.
	attributeRef := func(ref string) {
		parts := identifierRe.FindAllString(ref, -1)
		if len(parts) == 0 {
			return
		}
		col := parts[len(parts)-1]
		table := primaryTableFor(tables, "", col)
		if len(parts) >= 2 {
			table = parts[len(parts)-2]
			if real, ok := aliases[strings.ToLower(stripSchema(table))]; ok {
				table = real
			}
		}
		pushCandidate(table, col)
	}

	// Join columns are the highest-value targets.
	for _, m := range joinOnRe.FindAllStringSubmatch(query, -1) {
		pushCandidate(m[1], m[2]) // left side
		pushCandidate(m[3], m[4]) // right side
	}

	// Filter columns (WHERE x = / t.x = / LIKE / IN ...).
	for _, m := range whereColRe.FindAllStringSubmatch(query, -1) {
		attributeRef(m[1])
	}

	// ORDER BY / GROUP BY leading columns.
	for _, re := range []*regexp.Regexp{orderColRe, groupColRe} {
		for _, m := range re.FindAllStringSubmatch(query, -1) {
			for _, raw := range splitColumns(m[1]) {
				parts := strings.Fields(raw)
				if len(parts) == 0 {
					continue
				}
				attributeRef(parts[0])
			}
		}
	}

	var b strings.Builder
	b.WriteString("Index suggestions (heuristic — verify with EXPLAIN before creating):\n\n")
	suggestions := 0

	skippedUnknown := 0
	for _, table := range orderedKeys(candidates) {
		existing, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
		if err != nil {
			continue // cannot compare; skip silently
		}
		indexedText := strings.ToLower(fmt.Sprintf("%v", existing))
		knownCols, colErr := uc.tableColumns(ctx, dbID, strings.ToLower(dbType), table)

		for _, col := range candidates[table] {
			// When the catalog is readable, drop references that are not real
			// columns (ORDER BY aliases, expressions); when it is not, keep
			// the candidate rather than losing coverage.
			if colErr == nil && len(knownCols) > 0 && !knownCols[col] {
				skippedUnknown++
				continue
			}
			if indexCovers(indexedText, col) {
				continue
			}
			suggestions++
			fmt.Fprintf(&b, "  CREATE INDEX idx_%s_%s ON %s (%s);\n", table, col, table, col)
		}
	}

	if skippedUnknown > 0 {
		fmt.Fprintf(&b, "\n  (%d reference(s) ignored: not columns of their table)\n", skippedUnknown)
	}

	if suggestions == 0 {
		b.WriteString("  (none — filter/join/sort columns appear covered by existing indexes)\n")
	}
	return b.String(), nil
}

// extractQueryTables returns table names referenced by FROM/JOIN clauses,
// excluding subquery aliases.
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

func stripSchema(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

// primaryTableFor attributes an unqualified column to the first referenced
// table unless another table owns it via an explicit qualification nearby;
// heuristics stay intentionally simple.
func primaryTableFor(tables []string, _ string, _ string) string {
	if len(tables) == 0 {
		return ""
	}
	return stripSchema(tables[0])
}

func splitColumns(list string) []string {
	return strings.Split(list, ",")
}

// tableColumns returns the table's column names, lowercased. Callers treat a
// nil map (catalog unreadable) as "trust the extracted candidates".
func (uc *DatabaseUseCase) tableColumns(ctx context.Context, dbID, dbType, table string) (map[string]bool, error) {
	rows, err := uc.queryTableMetadata(ctx, dbID, columnQueries(dbType, table))
	if err != nil {
		return nil, err
	}
	cols := make(map[string]bool, len(rows))
	for _, row := range rows {
		for _, key := range []string{"column_name", "COLUMN_NAME", "Field", "name"} {
			if v, ok := row[key]; ok {
				if s, ok := v.(string); ok {
					cols[strings.ToLower(s)] = true
				}
				break
			}
		}
	}
	return cols, nil
}

// indexCovers checks whether any existing index definition names col as a
// whole identifier. Substring matching would treat "name" as covered by an
// index on "surname", so compare exact identifier tokens instead.
func indexCovers(existingIndexText, col string) bool {
	for _, tok := range identifierRe.FindAllString(existingIndexText, -1) {
		if strings.EqualFold(tok, col) {
			return true
		}
	}
	return false
}

func orderedKeys(m map[string][]string) []string {
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

// clauseKeywords may legally follow a table name inside FROM/JOIN clauses and
// must never be mistaken for that table's alias.
var clauseKeywords = map[string]bool{
	"where": true, "group": true, "order": true, "by": true, "having": true,
	"limit": true, "offset": true, "union": true, "except": true, "intersect": true,
	"left": true, "right": true, "inner": true, "outer": true, "cross": true,
	"natural": true, "full": true, "on": true, "using": true, "as": true,
	"set": true, "values": true, "returning": true, "window": true, "when": true,
}

// extractAliasMap maps query-local table aliases ("FROM books b") to their
// real table names so join and filter conditions written through aliases
// attribute candidates to actual tables.
func extractAliasMap(query string) map[string]string {
	out := map[string]string{}
	for _, m := range aliasRe.FindAllStringSubmatch(query, -1) {
		alias := strings.ToLower(m[2])
		if alias == "" || reservedWord(alias) || clauseKeywords[alias] || !isPlainIdentifier(alias) {
			continue
		}
		table := stripSchema(m[1])
		if _, dup := out[alias]; !dup && isPlainIdentifier(table) {
			out[alias] = table
		}
	}
	return out
}
