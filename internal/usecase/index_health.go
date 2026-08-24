package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// IndexHealth reports maintenance findings about a database's existing
// indexes: duplicates (same columns, same uniqueness) and redundant indexes
// (one is a leading-column prefix of another with compatible uniqueness).
// It is our counterpart of Postgres MCP Pro's analyze_db_health index
// findings, implemented engine-agnostically on top of the same catalog
// plumbing the index advisor uses.
func (uc *DatabaseUseCase) IndexHealth(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	dbType = strings.ToLower(dbType)

	tables, err := uc.allTables(ctx, dbID, dbType)
	if err != nil {
		return "", err
	}
	if len(tables) == 0 {
		return "No user tables found — nothing to analyze.", nil
	}

	var findings []string
	for _, table := range tables {
		rows, err := uc.queryTableMetadata(ctx, dbID, indexQueries(dbType, table))
		if err != nil || len(rows) == 0 {
			continue // no readable index metadata; skip silently like the advisor does
		}
		indexes := parseIndexRows(rows)
		findings = append(findings, redundancyFindings(dbType, table, indexes)...)
	}

	usageConsulted := false
	usage, ok := uc.usageFindings(ctx, dbID, dbType)
	if ok {
		usageConsulted = true
		findings = append(findings, usage...)
	}

	if len(findings) == 0 {
		return fmt.Sprintf("No duplicate or redundant indexes found across %d table(s).", len(tables)), nil
	}
	sort.Strings(findings)

	var b strings.Builder
	fmt.Fprintf(&b, "Index health findings across %d table(s), %d issue(s):\n\n", len(tables), len(findings))
	for _, f := range findings {
		b.WriteString("  " + f + "\n")
	}
	if usageConsulted {
		b.WriteString("\nUNUSED counts reflect scans since last statistics reset or server start;")
		b.WriteString("\na recently created index can legitimately show zero scans.")
	} else {
		b.WriteString("\nUsage-based findings need engine statistics")
		b.WriteString(" (pg_stat_user_indexes / sys.schema_unused_indexes); this view ran without them.")
	}
	return b.String(), nil
}

// usageFindings consults engine statistics catalogs for evidence the static
// structure pass cannot see: never-scanned indexes (PostgreSQL
// pg_stat_user_indexes, MySQL sys.schema_unused_indexes) and invalid
// indexes (PostgreSQL pg_index.indisvalid). Returns ok=false when the
// engine exposes no such statistics or every catalog read failed.
func (uc *DatabaseUseCase) usageFindings(ctx context.Context, dbID, dbType string) ([]string, bool) {
	var candidates []string
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		candidates = []string{
			"SELECT relname AS table_name, indexrelname AS index_name FROM pg_stat_user_indexes WHERE schemaname = 'public' AND idx_scan = 0",
			"SELECT i.indexrelid::regclass::text AS index_name FROM pg_index i WHERE NOT i.indisvalid",
			"SELECT relname AS table_name, n_live_tup, n_dead_tup FROM pg_stat_user_tables WHERE schemaname = 'public' AND n_dead_tup >= 1000 ORDER BY n_dead_tup DESC LIMIT 20",
		}
	case "mysql":
		candidates = []string{
			"SELECT table_name, index_name FROM sys.schema_unused_indexes",
			"SELECT table_name AS table_name, engine AS engine, data_free AS data_free FROM information_schema.tables WHERE data_free > 16777216 ORDER BY data_free DESC LIMIT 20",
		}
	default:
		return nil, false // SQLite and unknown engines: no statistics catalogs
	}

	var findings []string
	for _, q := range candidates {
		rows, err := uc.queryTableMetadata(ctx, dbID, []string{q})
		if err != nil {
			continue // statistics may not exist yet (e.g. no sys schema); skip that catalog
		}
		switch {
		case strings.Contains(q, "idx_scan"):
			findings = append(findings, formatUnusedFindings(rows)...)
		case strings.Contains(q, "indisvalid"):
			findings = append(findings, formatInvalidFindings(rows)...)
		case strings.Contains(q, "n_dead_tup"):
			findings = append(findings, formatBloatFindings(rows)...)
		case strings.Contains(q, "data_free"):
			findings = append(findings, formatMySQLBloatFindings(rows)...)
		default:
			findings = append(findings, formatMySQLUnusedFindings(rows)...)
		}
	}
	return findings, true
}

// formatUnusedFindings renders PostgreSQL never-scanned index rows.
func formatUnusedFindings(rows []map[string]interface{}) []string {
	var out []string
	for _, r := range rows {
		name := rowString(r, "index_name", "indexrelname", "INDEX_NAME")
		table := rowString(r, "table_name", "relname", "TABLE_NAME")
		if name == "" {
			continue
		}
		if table != "" {
			out = append(out, fmt.Sprintf("UNUSED on %s: %s has zero scans since statistics were last reset:\n    DROP INDEX %s;", table, name, name))
		} else {
			out = append(out, fmt.Sprintf("UNUSED: %s has zero scans since statistics were last reset:\n    DROP INDEX %s;", name, name))
		}
	}
	return out
}

// formatInvalidFindings renders PostgreSQL invalid-index rows (failed
// CREATE INDEX CONCURRENTLY leaves these behind).
func formatInvalidFindings(rows []map[string]interface{}) []string {
	var out []string
	for _, r := range rows {
		if name := rowString(r, "index_name", "INDEX_NAME"); name != "" {
			out = append(out, fmt.Sprintf("INVALID: %s is marked invalid (often a leftover from a failed CREATE INDEX CONCURRENTLY):\n    DROP INDEX %s; -- then recreate if still needed", name, name))
		}
	}
	return out
}

// formatMySQLUnusedFindings renders sys.schema_unused_indexes rows.
func formatMySQLUnusedFindings(rows []map[string]interface{}) []string {
	var out []string
	for _, r := range rows {
		name := rowString(r, "index_name", "INDEX_NAME")
		table := rowString(r, "table_name", "TABLE_NAME")
		if name == "" || table == "" {
			continue
		}
		out = append(out, fmt.Sprintf("UNUSED on %s: %s has zero reads since the server started:\n    ALTER TABLE `%s` DROP INDEX `%s`;", table, name, table, name))
	}
	return out
}

// rowString returns the first non-empty string among the given keys.
func rowString(r map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := r[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// rowInt coerces common numeric cell types to int; missing cells read as 0.
func rowInt(r map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		v, ok := r[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case int64:
			return n
		case int:
			return int64(n)
		case float64:
			return int64(n)
		}
	}
	return 0
}

// bloatThreshold is the dead-to-live ratio above which a table is flagged,
// alongside a floor so tiny tables with one or two dead rows stay quiet.
const (
	bloatRatioFloor    = 0.20
	bloatMinDeadTuples = 1000
)

// formatBloatFindings renders PostgreSQL dead-tuple findings from
// pg_stat_user_tables. High dead ratios mean autovacuum is falling behind;
// the remedy is usually VACUUM ANALYZE, not index changes.
func formatBloatFindings(rows []map[string]interface{}) []string {
	var out []string
	for _, r := range rows {
		table := rowString(r, "table_name", "relname")
		live := rowInt(r, "n_live_tup")
		dead := rowInt(r, "n_dead_tup")
		if table == "" || dead < bloatMinDeadTuples {
			continue
		}
		total := live + dead
		if total > 0 && float64(dead)/float64(total) < bloatRatioFloor {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = float64(dead) / float64(total) * 100
		}
		out = append(out, fmt.Sprintf(
			"BLOAT on %s: %d dead of %d tuple(s) (%.1f%%) — autovacuum may be behind:\n    VACUUM (ANALYZE) %s;",
			table, dead, total, pct, table))
	}
	return out
}

// formatMySQLBloatFindings renders information_schema DATA_FREE findings.
// DATA_FREE is a coarse signal (InnoDB reports free extents per tablespace);
// it flags candidates for OPTIMIZE TABLE, not certainties.
func formatMySQLBloatFindings(rows []map[string]interface{}) []string {
	var out []string
	for _, r := range rows {
		table := rowString(r, "table_name", "TABLE_NAME")
		engine := rowString(r, "engine", "ENGINE")
		free := rowInt(r, "data_free", "DATA_FREE")
		if table == "" || free <= 0 {
			continue
		}
		note := ""
		if engine != "" {
			note = fmt.Sprintf(" [%s]", engine)
		}
		out = append(out, fmt.Sprintf(
			"FRAGMENTATION on %s%s: %.1f MB reported free — candidate for OPTIMIZE TABLE if growth churn is high:",
			table, note, float64(free)/(1024*1024)))
	}
	return out
}

// allTables enumerates user tables using the first candidate query that
// returns rows.
func (uc *DatabaseUseCase) allTables(ctx context.Context, dbID, dbType string) ([]string, error) {
	var candidates []string
	switch dbType {
	case "postgres", "timescale", "timescaledb":
		candidates = []string{
			"SELECT table_name AS table_name FROM information_schema.tables WHERE table_schema = 'public'",
			"SELECT table_name FROM pg_catalog.pg_tables WHERE schemaname = 'public'",
		}
	case "mysql":
		candidates = []string{"SHOW TABLES"}
	case "sqlite", "sqlite3":
		candidates = []string{
			"SELECT name AS table_name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'",
		}
	default:
		candidates = []string{
			"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'",
			"SELECT table_name FROM information_schema.tables",
		}
	}
	rows, err := uc.queryTableMetadata(ctx, dbID, candidates)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		for _, key := range []string{"table_name", "TABLE_NAME", "Tables_in_" /* MySQL SHOW TABLES prefix */} {
			for k, v := range r {
				if k == key || strings.HasPrefix(k, key) {
					if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
						out = append(out, s)
					}
				}
			}
		}
	}
	return out, nil
}

// parsedIndex describes one index as the health analyzer sees it.
type parsedIndex struct {
	name   string
	cols   []string
	unique bool
}

// parseIndexRows converts raw catalog rows into indexes. Handles both row
// shapes: definition-style (SQLite, PostgreSQL) and one-row-per-column
// style (MySQL SHOW INDEX). Definition-less rows (autoindexes) are skipped.
func parseIndexRows(rows []map[string]interface{}) []parsedIndex {
	byName := map[string]*parsedIndex{}
	order := []string{}
	get := func(name string) *parsedIndex {
		if byName[name] == nil {
			byName[name] = &parsedIndex{name: name}
			order = append(order, name)
		}
		return byName[name]
	}

	for _, r := range rows {
		str := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := r[k]; ok {
					if s, ok := v.(string); ok {
						return s
					}
				}
			}
			return ""
		}

		def := str("definition", "DEFINITION", "Definition")
		name := str("index_name", "INDEX_NAME", "IndexName", "Key_name")
		if def != "" {
			if idx, ok := parseCreateIndex(def); ok {
				if name != "" {
					idx.name = name // prefer the catalog's name column when present
				}
				if idx.name != "" {
					*get(idx.name) = *idx
				}
			}
			continue // unparseable, auto-generated, or nameless definition
		}

		// MySQL shape: Key_name / Seq_in_index / Column_name / Non_unique.
		col := str("Column_name", "COLUMN_NAME")
		if name == "" || col == "" {
			continue
		}
		idx := get(name)
		idx.cols = append(idx.cols, strings.ToLower(col))
		if str("Non_unique") == "0" {
			idx.unique = true
		}
	}

	// MySQL rows may arrive out of order; sort each index's columns by
	// Seq_in_index when present.
	for _, name := range order {
		if seqs, ok := seqOrder(rows, name); ok {
			cols := byName[name].cols
			sorted := make([]string, len(cols))
			for i, si := range seqs {
				if si >= 1 && si <= len(cols) {
					sorted[si-1] = cols[i]
				}
			}
			byName[name].cols = sorted
		}
	}

	out := make([]parsedIndex, 0, len(order))
	for _, name := range order {
		if len(byName[name].cols) > 0 {
			out = append(out, *byName[name])
		}
	}
	return out
}

// seqOrder extracts Seq_in_index values for an index's rows in arrival
// order; ok=false when absent (non-MySQL shapes).
func seqOrder(rows []map[string]interface{}, indexName string) ([]int, bool) {
	var seqs []int
	for _, r := range rows {
		if n, ok := r["Key_name"].(string); !ok || n != indexName {
			continue
		}
		switch v := r["Seq_in_index"].(type) {
		case int64:
			seqs = append(seqs, int(v))
		case int:
			seqs = append(seqs, v)
		case float64:
			seqs = append(seqs, int(v))
		default:
			return nil, false
		}
	}
	return seqs, len(seqs) > 0
}

// parseCreateIndex pulls columns and uniqueness from a CREATE INDEX
// statement (SQLite and PostgreSQL indexdef shapes).
func parseCreateIndex(def string) (*parsedIndex, bool) {
	head := strings.ToUpper(def)
	if len(head) > 32 {
		head = head[:32]
	}
	idx := &parsedIndex{unique: strings.Contains(head, "UNIQUE")}
	open := strings.LastIndex(def, "(")
	close := strings.LastIndex(def, ")")
	if open < 0 || close <= open {
		return nil, false
	}
	for _, part := range strings.Split(def[open+1:close], ",") {
		col := strings.TrimSpace(part)
		// Trim collations, directions, and functional expressions are left
		// as-is; only plain identifiers count toward redundancy checks.
		if i := strings.IndexAny(col, " ()"); i >= 0 {
			col = col[:i]
		}
		col = strings.ToLower(col)
		if col != "" && isPlainIdentifier(col) {
			idx.cols = append(idx.cols, col)
		}
	}
	return idx, len(idx.cols) > 0
}

// redundancyFindings compares a table's indexes. Duplicate groups are
// canonicalized to their smallest name FIRST so the prefix-redundancy pass
// runs over one representative per column signature — otherwise overlapping
// indexes produce contradictory "keep X / drop X" advice.
func redundancyFindings(dbType, table string, indexes []parsedIndex) []string {
	sorted := make([]parsedIndex, len(indexes))
	copy(sorted, indexes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })

	var findings []string
	var canonical []parsedIndex
	for _, ix := range sorted {
		dup := false
		for _, c := range canonical {
			if sameCols(c.cols, ix.cols) && c.unique == ix.unique {
				findings = append(findings,
					fmt.Sprintf("DUPLICATE on %s: %s duplicates %s (%s). Keep one:\n    %s",
						table, ix.name, c.name, strings.Join(ix.cols, ", "), dropStatement(dbType, table, ix.name)))
				dup = true
				break
			}
		}
		if !dup {
			canonical = append(canonical, ix)
		}
	}

	for _, a := range canonical {
		for _, b2 := range canonical {
			if a.name == b2.name {
				continue
			}
			// Redundant prefix: b's columns start with a's and b offers no
			// stronger guarantee, so queries served by `a` can use `b`.
			if len(a.cols) < len(b2.cols) && prefixOf(a.cols, b2.cols) &&
				(!a.unique || b2.unique) {
				findings = append(findings,
					fmt.Sprintf("REDUNDANT on %s: %s (%s) is a prefix of %s (%s); the larger index can serve its queries:\n    %s",
						table, a.name, strings.Join(a.cols, ", "), b2.name, strings.Join(b2.cols, ", "), dropStatement(dbType, table, a.name)))
			}
		}
	}
	return findings
}

func sameCols(a, b []string) bool {
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

func prefixOf(prefix, full []string) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if prefix[i] != full[i] {
			return false
		}
	}
	return true
}

// dropStatement renders engine-appropriate drop syntax for the common
// cases; unknown engines fall back to plain DROP INDEX.
func dropStatement(dbType, table, indexName string) string {
	switch dbType {
	case "mysql":
		return fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", table, indexName)
	case "postgres", "timescale", "timescaledb", "sqlite", "sqlite3":
		return fmt.Sprintf("DROP INDEX %s;", indexName)
	default:
		return fmt.Sprintf("DROP INDEX %s;", indexName)
	}
}
