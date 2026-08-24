package usecase

import (
	"context"
	"fmt"
	"strings"
)

// indexCandidate is one proposed index awaiting plan validation.
type indexCandidate struct {
	table string
	cols  []string
}

// candidatesForQuery mirrors emitIndexSuggestions' candidate logic as
// structured data: one composite per table (equality columns first, sort
// appended, capped at 3 columns) plus single-column candidates from range
// predicates and join keys that the composite does not already lead with.
// The validator needs the same proposals suggest_indexes would print so a
// USED/NOT USED verdict always refers to something the advisor shows.
func candidatesForQuery(advice map[string]*tableAdvice) []indexCandidate {
	var out []indexCandidate
	for table, a := range advice {
		comp := mergeDedup(a.eq, a.sort)
		hasComp := false
		if len(comp) >= 2 {
			if len(comp) > 3 {
				comp = comp[:3]
			}
			out = append(out, indexCandidate{table: table, cols: comp})
			hasComp = true
		}
		for _, list := range [][]string{a.eq, a.rng, a.join} {
			for _, col := range list {
				if hasComp && col == comp[0] {
					continue // an emitted composite already leads with this column
				}
				out = append(out, indexCandidate{table: table, cols: []string{col}})
			}
		}
	}
	return out
}

// ValidateIndexSuggestions closes the gap to Postgres MCP Pro's
// hypothetical-index tuning: instead of asking the user to verify
// heuristic suggestions manually, PostgreSQL's hypopg extension installs
// them cost-free (no disk writes, planner-visible only for this session)
// and EXPLAIN reveals whether the planner would actually pick each one.
// Other engines have no hypopg equivalent and get an honest refusal.
func (uc *DatabaseUseCase) ValidateIndexSuggestions(ctx context.Context, dbID, query string) (string, error) {
	query = strings.TrimSpace(strings.TrimRight(query, ";"))
	if query == "" {
		return "", fmt.Errorf("query parameter must not be empty")
	}

	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	dbType = strings.ToLower(dbType)
	switch dbType {
	case "postgres", "timescale", "timescaledb":
	default:
		return fmt.Sprintf("Hypothetical-index validation requires PostgreSQL's hypopg extension; %q has no equivalent mechanism. Use action=suggest_indexes heuristics plus manual EXPLAIN instead.", dbType), nil
	}

	advice := extractIndexAdvice(query)
	if len(advice) == 0 {
		return "No tables detected in the query; nothing to validate.", nil
	}
	cands := candidatesForQuery(advice)
	if len(cands) == 0 {
		return "No index-worthy access patterns found in the query.", nil
	}

	// Best-effort enablement, then verify presence — CREATE EXTENSION may
	// fail on privileges or a missing package, which degrades to guidance.
	if _, err := uc.ExecuteStatement(ctx, dbID, "CREATE EXTENSION IF NOT EXISTS hypopg", nil); err != nil {
		_ = err // fall through to the presence check
	}
	check, err := uc.ExecuteQuery(ctx, dbID, `SELECT count(*) AS n FROM pg_extension WHERE extname = 'hypopg'`, nil)
	if err != nil || !strings.Contains(check, " 1") {
		return ("The hypopg extension is not available on this database. Install it with:\n" +
			"  CREATE EXTENSION hypopg;\n" +
			"(package e.g. postgresql-15-hypopg / postgresql-16-hypopg from PGDG), then retry."), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Hypothetical-index validation for:\n  %s\n\n", strings.ReplaceAll(query, "\n", " "))
	b.WriteString("(hypopg indexes are planner-visible only and vanish on reset — nothing is written)\n")

	used := 0
	defer func() { _, _ = uc.ExecuteQuery(ctx, dbID, "SELECT hypopg_reset()", nil) }() //nolint:errcheck // best-effort cleanup; hypotheticals vanish with the session anyway
	for _, c := range cands {
		_, _ = uc.ExecuteQuery(ctx, dbID, "SELECT hypopg_reset()", nil) //nolint:errcheck // best-effort; verdicts stay independent

		createSQL := fmt.Sprintf("CREATE INDEX ON %s (%s)", c.table, strings.Join(c.cols, ", "))
		res, err := uc.ExecuteQuery(ctx, dbID,
			fmt.Sprintf("SELECT indexname FROM hypopg_create_index('%s')", strings.ReplaceAll(createSQL, "'", "''")), nil)
		if err != nil {
			fmt.Fprintf(&b, "  ? (%s): could not create hypothetical index: %v\n", createSQL, err)
			continue
		}
		name := hypoIndexName(res)
		if name == "" {
			fmt.Fprintf(&b, "  ? (%s): hypopg returned no index name\n", createSQL)
			continue
		}

		plan, err := uc.ExecuteExplain(ctx, dbID, query, false)
		if err != nil {
			fmt.Fprintf(&b, "  ? (%s): EXPLAIN failed: %v\n", createSQL, err)
			continue
		}
		if strings.Contains(strings.ToLower(plan), strings.ToLower(name)) {
			used++
			fmt.Fprintf(&b, "  ✓ USED   %s\n      the planner picks this hypothetical index\n", createSQL)
		} else {
			fmt.Fprintf(&b, "  ✗ UNUSED %s\n      the planner ignores it — creating it would buy nothing for this query\n", createSQL)
		}
	}

	fmt.Fprintf(&b, "\n%d of %d candidate(s) validated as used by the planner.\n", used, len(cands))
	return b.String(), nil
}

// hypoIndexName extracts the indexname cell from a hypopg_create_index
// result rendered through standard row formatting ("Results:" header,
// column names, a dash rule, then tab-joined data rows and footers).
func hypoIndexName(res string) string {
	lines := strings.Split(res, "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, "---") {
			continue
		}
		for _, d := range lines[i+1:] {
			d = strings.TrimSpace(d)
			if d == "" || strings.HasPrefix(d, "Total rows") || strings.HasPrefix(d, "Truncated") {
				continue
			}
			f := strings.Fields(d)
			if len(f) > 0 {
				return f[0]
			}
		}
		break // only one dash-rule section expected for this query shape
	}
	return ""
}
