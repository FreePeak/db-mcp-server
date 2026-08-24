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

// tableAdvice holds the candidate index columns extracted from one SQL
// statement, grouped by predicate class.
type tableAdvice struct {
	eq   []string // equality-filterable: =, IN, LIKE/ILIKE
	rng  []string // range predicates: >, <, >=, <=, BETWEEN
	sort []string // ORDER BY / GROUP BY leading columns
	join []string // join keys
}

func (t *tableAdvice) add(class, col string) {
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

// extractIndexAdvice parses a SQL statement into per-table candidate columns
// grouped by predicate class. Returns an empty map when no tables appear.
func extractIndexAdvice(query string) map[string]*tableAdvice {
	advice := map[string]*tableAdvice{}
	tables := extractQueryTables(query)
	if len(tables) == 0 {
		return advice // DML without FROM/JOIN offers no index surface
	}
	tableSet := make(map[string]bool, len(tables))
	for _, t := range tables {
		tableSet[strings.ToLower(t)] = true
	}
	aliases := extractAliasMap(query)

	entryFor := func(table string) *tableAdvice {
		if advice[table] == nil {
			advice[table] = &tableAdvice{}
		}
		return advice[table]
	}

	// push resolves the owning table (aliases included) and files the column
	// under its predicate class. Columns resolving outside the statement's
	// tables are dropped rather than inventing phantom targets.
	push := func(rawTable, col, class string) {
		table := stripSchema(rawTable)
		if real, ok := aliases[strings.ToLower(table)]; ok {
			table = real // join aliases must resolve to actual tables
		}
		if !tableSet[strings.ToLower(table)] {
			return
		}
		col = strings.ToLower(col)
		if !isPlainIdentifier(col) || reservedWord(col) {
			return
		}
		entryFor(table).add(class, col)
	}

	// attributeRef resolves a possibly-qualified column reference
	// (b.author_id): explicit qualifiers go through the alias map, bare
	// columns fall back to the first referenced table.
	attributeRef := func(ref, class string) {
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
		push(table, col, class)
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
		attributeRef(m[1], class)
	}

	// ORDER BY / GROUP BY leading columns.
	for _, re := range []*regexp.Regexp{orderColRe, groupColRe} {
		for _, m := range re.FindAllStringSubmatch(query, -1) {
			for _, raw := range splitColumns(m[1]) {
				parts := strings.Fields(raw)
				if len(parts) == 0 {
					continue
				}
				attributeRef(parts[0], "sort")
			}
		}
	}
	return advice
}

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

	advice := extractIndexAdvice(query)
	if len(advice) == 0 {
		return "No tables detected in the query; nothing to advise on.", nil
	}

	return uc.emitIndexSuggestions(ctx, dbID, dbType, []statementAdvice{{advice: advice, weight: 1}},
		"Index suggestions (heuristic — verify with EXPLAIN before creating):", false, "statement(s)"), nil
}

// statementAdvice pairs one analyzed statement's extracted advice with how
// many executions it represents.
type statementAdvice struct {
	advice map[string]*tableAdvice
	weight int
}

// emitIndexSuggestions renders CREATE INDEX proposals for the given per-
// statement advice. Composites form within a single statement only — folding
// together columns from disjoint statements would propose indexes whose
// trailing columns serve nothing. Across statements, identical composites
// coalesce, each line's weight counts every analyzed execution whose columns
// it serves, and output is ranked by traffic.
func (uc *DatabaseUseCase) emitIndexSuggestions(ctx context.Context, dbID, dbType string, entries []statementAdvice, header string, annotate bool, weightUnit string) string {
	const (
		cEq = iota
		cRng
		cSort
		cJoin
		classCount
	)

	var b strings.Builder
	b.WriteString(header + "\n\n")
	suggestions := 0
	skippedUnknown := 0

	// Composite candidates keyed by their column signature, formed per
	// statement and coalesced across statements.
	type compKey struct {
		table string
		cols  string
	}
	comps := map[compKey][]string{}
	compOrder := []compKey{}

	// Merge single-column candidates per table and class, summing execution
	// weights.
	merged := map[string]*[classCount][]colHit{}
	totalWeight := 0
	for _, e := range entries {
		totalWeight += e.weight
		for table, a := range e.advice {
			if comp := mergeDedup(a.eq, a.sort); len(comp) >= 2 {
				if len(comp) > 3 {
					comp = comp[:3]
				}
				k := compKey{table: table, cols: strings.Join(comp, ",")}
				if _, seen := comps[k]; !seen {
					comps[k] = comp
					compOrder = append(compOrder, k)
				}
			}
			m := merged[table]
			if m == nil {
				m = &[classCount][]colHit{}
				merged[table] = m
			}
			for ci, list := range [][]string{a.eq, a.rng, a.sort, a.join} {
				for _, col := range list {
					found := false
					for i := range m[ci] {
						if m[ci][i].col == col {
							m[ci][i].hits += e.weight
							found = true
							break
						}
					}
					if !found {
						m[ci] = append(m[ci], colHit{col: col, hits: e.weight})
					}
				}
			}
		}
	}

	// servingWeight sums the executions of all statements whose eq/sort
	// columns are a subset of cols — those are the queries an index on cols
	// can actually serve.
	servingWeight := func(table string, cols map[string]bool) int {
		w := 0
		for _, e := range entries {
			a := e.advice[table]
			if a == nil || len(a.eq)+len(a.sort) == 0 {
				continue
			}
			subset := true
			for _, c := range a.eq {
				if !cols[c] {
					subset = false
					break
				}
			}
			if subset {
				for _, c := range a.sort {
					if !cols[c] {
						subset = false
						break
					}
				}
			}
			if subset {
				w += e.weight
			}
		}
		return w
	}

	rankStable := func(list []colHit) {
		sort.SliceStable(list, func(i, j int) bool { return list[i].hits > list[j].hits })
	}

	// Rank composite candidates by served traffic.
	type rankedComp struct {
		key    compKey
		cols   []string
		weight int
	}
	rankedComps := make([]rankedComp, 0, len(compOrder))
	for _, k := range compOrder {
		set := make(map[string]bool, len(comps[k]))
		for _, c := range comps[k] {
			set[c] = true
		}
		rankedComps = append(rankedComps, rankedComp{key: k, cols: comps[k], weight: servingWeight(k.table, set)})
	}
	sort.SliceStable(rankedComps, func(i, j int) bool { return rankedComps[i].weight > rankedComps[j].weight })

	note := func(hits int) string {
		if !annotate || totalWeight <= 1 {
			return ""
		}
		return fmt.Sprintf("  -- serves %d of %d %s", hits, totalWeight, weightUnit)
	}

	for _, rc := range rankedComps {
		table := rc.key.table
		existing, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
		if err != nil {
			continue // cannot compare; skip silently
		}
		indexedText := strings.ToLower(fmt.Sprintf("%v", existing))
		ccols := uc.constraintCoveredCols(ctx, dbID, strings.ToLower(dbType), table)

		cols := make([]string, 0, len(rc.cols))
		valid := true
		for _, c := range rc.cols {
			if knownCols, colErr := uc.tableColumns(ctx, dbID, strings.ToLower(dbType), table); colErr != nil || knownCols[c] {
				cols = append(cols, c)
			} else {
				skippedUnknown++
				valid = false
			}
		}
		if valid && len(cols) >= 2 && !indexCovers(indexedText, cols[0]) && !ccols[cols[0]] {
			suggestions++
			name := append([]string{table}, cols...)
			fmt.Fprintf(&b, "  CREATE INDEX idx_%s ON %s (%s);%s\n",
				strings.Join(name, "_"), table, strings.Join(cols, ", "), note(rc.weight))
		}
	}

	keys := make([]string, 0, len(merged))
	for t := range merged {
		keys = append(keys, t)
	}
	sort.Strings(keys)

	for _, table := range keys {
		m := merged[table]
		existing, err := uc.queryTableMetadata(ctx, dbID, indexQueries(strings.ToLower(dbType), table))
		if err != nil {
			continue // cannot compare; skip silently
		}
		indexedText := strings.ToLower(fmt.Sprintf("%v", existing))
		ccols := uc.constraintCoveredCols(ctx, dbID, strings.ToLower(dbType), table)
		knownCols, colErr := uc.tableColumns(ctx, dbID, strings.ToLower(dbType), table)

		handled := map[string]bool{}
		for _, rc := range rankedComps {
			if rc.key.table == table {
				for _, c := range rc.cols {
					handled[c] = true
				}
			}
		}

		// Drop references that are not real columns when the catalog is
		// readable (ORDER BY aliases, expressions); when it is not, keep the
		// candidates rather than losing coverage. Lists stay ranked by the
		// executions they serve.
		filter := func(list []colHit) []colHit {
			out := make([]colHit, 0, len(list))
			for _, ch := range list {
				if colErr == nil && len(knownCols) > 0 && !knownCols[ch.col] {
					skippedUnknown++
					continue
				}
				out = append(out, ch)
			}
			rankStable(out)
			return out
		}
		eq, rng, srt, jn := filter(m[cEq]), filter(m[cRng]), filter(m[cSort]), filter(m[cJoin])

		for _, list := range [][]colHit{eq, rng, srt, jn} {
			for _, ch := range list {
				if handled[ch.col] || indexCovers(indexedText, ch.col) || ccols[ch.col] {
					continue
				}
				suggestions++
				fmt.Fprintf(&b, "  CREATE INDEX idx_%s_%s ON %s (%s);%s\n",
					table, ch.col, table, ch.col, note(ch.hits))
			}
		}
	}

	if skippedUnknown > 0 {
		fmt.Fprintf(&b, "\n  (%d reference(s) ignored: not columns of their table)\n", skippedUnknown)
	}

	if suggestions == 0 {
		b.WriteString("  (none — filter/join/sort columns appear covered by existing indexes)\n")
	}
	return b.String()
}

// colHit tracks a candidate column and how many analyzed statements use it.
type colHit struct {
	col  string
	hits int
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

// constraintCoveredCols returns columns already enforced unique by
// PRIMARY KEY or UNIQUE constraints — indexes the engine maintains
// automatically, so proposing one would duplicate work. This complements
// indexQueries: SQLite autoindexes carry NULL sql in sqlite_master and
// never appear in definition-based index listings.
func (uc *DatabaseUseCase) constraintCoveredCols(ctx context.Context, dbID, dbType, table string) map[string]bool {
	rows, err := uc.queryTableMetadata(ctx, dbID, constraintQueries(dbType, table))
	if err != nil {
		return nil // cannot read constraints; suggest as before rather than blind
	}
	out := map[string]bool{}
	for _, r := range rows {
		switch strings.ToUpper(rowString(r, "constraint_type", "CONSTRAINT_TYPE")) {
		case "PRIMARY KEY", "UNIQUE":
			if col := strings.ToLower(rowString(r, "column_name", "COLUMN_NAME")); col != "" {
				out[col] = true
			}
		}
	}
	return out
}
