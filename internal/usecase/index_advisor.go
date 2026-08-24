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
	whereColRe   = regexp.MustCompile(`(?i)(?:WHERE|AND|OR)\s+\(?((?:[A-Za-z_][A-Za-z0-9_$]*)(?:\.[A-Za-z_][A-Za-z0-9_$]*)?)\s*(!=|<>|>=|<=|=|>|<|ILIKE|LIKE|IN\b|BETWEEN\b)`)
	aliasRe      = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_][A-Za-z0-9_$.]*)(?:\s+(?:AS\s+)?([A-Za-z_][A-Za-z0-9_$]*))?`)
	orderColRe   = regexp.MustCompile(`(?i)\bORDER\s+BY\s+((?:[A-Za-z_][A-Za-z0-9_.]*(?:\s+(?:ASC|DESC))?(?:\s*,\s*)?)+)`)
	groupColRe   = regexp.MustCompile(`(?i)\bGROUP\s+BY\s+((?:[A-Za-z_][A-Za-z0-9_.]*(?:\s*,\s*)?)+)`)
	identifierRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// SuggestIndexes compares the columns a query filters, joins, and sorts on
// against the database's existing indexes, proposing CREATE INDEX statements
// for uncovered high-value columns. Equality predicates and sort columns are
// folded into one composite candidate per table (equality columns first, the
// industry-standard btree ordering); range predicates and join keys stay
// single-column. Heuristic by design — every suggestion is labelled as such
// and EXPLAIN should confirm before acting.
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

	type tableAdvice struct {
		eq   []string // equality-filterable: =, IN, LIKE/ILIKE
		rng  []string // range predicates: >, <, >=, <=, BETWEEN
		sort []string // ORDER BY / GROUP BY leading columns
		join []string // join keys
	}
	add := func(t *tableAdvice, class, col string) {
		var dst *[]string
		switch class {
		case "eq":
			dst = &t.eq
		case "rng":
			dst = &t.rng
		case "sort":
			dst = &t.sort
		default:
			dst = &t.join
		}
		for _, c := range *dst {
			if c == col {
				return
			}
		}
		*dst = append(*dst, col)
	}

	advice := map[string]*tableAdvice{}
	entryFor := func(table string) *tableAdvice {
		if advice[table] == nil {
			advice[table] = &tableAdvice{}
		}
		return advice[table]
	}

	// push resolves the owning table (aliases included) and files the column
	// under its predicate class.
	push := func(rawTable, col, class string) {
		table := stripSchema(rawTable)
		if real, ok := aliases[strings.ToLower(table)]; ok {
			table = real // join aliases must resolve to actual tables
		}
		col = strings.ToLower(col)
		if !isPlainIdentifier(col) || reservedWord(col) {
			return
		}
		add(entryFor(table), class, col)
	}

	// attributeWhere resolves a possibly-qualified filter reference
	// (b.author_id): explicit qualifiers go through the alias map, bare
	// columns fall back to the first referenced table.
	attributeWhere := func(ref, opClass string) {
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
		push(table, col, opClass)
	}

	// Join columns are their own access path — never folded into composites.
	for _, m := range joinOnRe.FindAllStringSubmatch(query, -1) {
		push(m[1], m[2], "join") // left side
		push(m[3], m[4], "join") // right side
	}

	// Filter columns classified by operator: equality-shaped (=, IN, LIKE)
	// can share a composite; range shapes cannot.
	for _, m := range whereColRe.FindAllStringSubmatch(query, -1) {
		class := "rng"
		switch strings.ToUpper(m[2]) {
		case "=", "IN", "LIKE", "ILIKE":
			class = "eq"
		}
		attributeWhere(m[1], class)
	}

	// ORDER BY / GROUP BY leading columns.
	for _, re := range []*regexp.Regexp{orderColRe, groupColRe} {
		for _, m := range re.FindAllStringSubmatch(query, -1) {
			for _, raw := range splitColumns(m[1]) {
				parts := strings.Fields(raw)
				if len(parts) == 0 {
					continue
				}
				refParts := identifierRe.FindAllString(parts[0], -1)
				if len(refParts) == 0 {
					continue
				}
				col := refParts[len(refParts)-1]
				table := primaryTableFor(tables, "", col)
				if len(refParts) >= 2 {
					table = refParts[len(refParts)-2]
					if real, ok := aliases[strings.ToLower(stripSchema(table))]; ok {
						table = real
					}
				}
				push(table, col, "sort")
			}
		}
	}

	var b strings.Builder
	b.WriteString("Index suggestions (heuristic — verify with EXPLAIN before creating):\n\n")
	suggestions := 0
	skippedUnknown := 0

	keys := make([]string, 0, len(advice))
	for t := range advice {
		keys = append(keys, t)
	}
	sort.Strings(keys)

	// validColumns drops references that are not real columns when the
	// catalog is readable (ORDER BY aliases, expressions); when it is not,
	// candidates are kept rather than losing coverage.
	validColumns := func(knownCols map[string]bool, colErr bool, cols []string) []string {
		out := make([]string, 0, len(cols))
		for _, c := range cols {
			if !colErr && len(knownCols) > 0 && !knownCols[c] {
				skippedUnknown++
				continue
			}
			out = append(out, c)
		}
		return out
	}

	for _, table := range keys {
		a := advice[table]
		existing, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
		if err != nil {
			continue // cannot compare; skip silently
		}
		indexedText := strings.ToLower(fmt.Sprintf("%v", existing))
		knownCols, colErr := uc.tableColumns(ctx, dbID, strings.ToLower(dbType), table)
		colFailed := colErr != nil

		eq := validColumns(knownCols, colFailed, a.eq)
		rng := validColumns(knownCols, colFailed, a.rng)
		srt := validColumns(knownCols, colFailed, a.sort)
		jn := validColumns(knownCols, colFailed, a.join)

		handled := map[string]bool{}
		// Composite candidate: equality columns then sort columns, capped at
		// three. Usable only when its leading column is uncovered.
		composite := mergeDedup(eq, srt)
		if len(composite) > 3 {
			composite = composite[:3]
		}
		if len(composite) >= 2 && !indexCovers(indexedText, composite[0]) {
			suggestions++
			name := append([]string{table}, composite...)
			fmt.Fprintf(&b, "  CREATE INDEX idx_%s ON %s (%s);\n",
				strings.Join(name, "_"), table, strings.Join(composite, ", "))
			for _, c := range composite {
				handled[c] = true
			}
		}
		for _, group := range [][]string{eq, rng, srt, jn} {
			for _, c := range group {
				if handled[c] || indexCovers(indexedText, c) {
					continue
				}
				suggestions++
				fmt.Fprintf(&b, "  CREATE INDEX idx_%s_%s ON %s (%s);\n", table, c, table, c)
			}
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

// mergeDedup appends srcs to dst preserving order and dropping duplicates.
func mergeDedup(dst []string, srcs ...[]string) []string {
	for _, src := range srcs {
		for _, v := range src {
			dup := false
			for _, e := range dst {
				if e == v {
					dup = true
					break
				}
			}
			if !dup {
				dst = append(dst, v)
			}
		}
	}
	return dst
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
