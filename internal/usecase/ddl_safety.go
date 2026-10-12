package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Statement risk analysis: engine-agnostic static pre-flight for the execute
// tool. `dry_run` reports what a statement WOULD do — kind, risk level,
// actionable advisories — without touching the database, and real executions
// of high-risk statements carry the same advisories in their result so a
// transcript shows what just happened.
//
// This is deliberately the same layer as the read-only classifier: literal
// stripping first, then keyword classification, so string content cannot skew
// a verdict. No SQL parser, no per-engine special cases.

// RiskReport is the structured verdict for one statement (or a batch).
type RiskReport struct {
	Kind         string   `json:"kind"`          // read | write | ddl | destructive
	Risk         string   `json:"risk"`          // low | medium | high | critical
	MissingWhere bool     `json:"missing_where"` // UPDATE/DELETE with no WHERE
	Statements   int      `json:"statements"`    // statements in the batch
	Notes        []string `json:"notes"`
}

// riskOrder ranks verdicts; worstRisk keeps the maximum across a batch.
var riskOrder = map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

func worstRisk(a, b string) string {
	if riskOrder[strings.ToLower(a)] >= riskOrder[strings.ToLower(b)] {
		return a
	}
	return b
}

func worstKind(a, b string) string {
	order := map[string]int{"read": 0, "ddl": 1, "write": 2, "destructive": 3}
	if order[b] > order[a] {
		return b
	}
	return a
}

// splitStatements splits a literal-stripped batch on top-level semicolons.
func splitStatements(stripped string) []string {
	parts := strings.Split(stripped, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return out
}

// classifySingle maps one statement's leading keyword to kind + base risk.
func classifySingle(stmt string) (kind, risk string, notes []string) {
	upper := strings.ToUpper(stmt)
	first := upper
	if i := strings.IndexAny(upper, " \t\n("); i >= 0 {
		first = upper[:i]
	}
	switch first {
	case "SELECT", "WITH", "SHOW", "EXPLAIN", "DESCRIBE", "PRAGMA":
		return "read", "low", nil
	case "INSERT":
		return "write", "medium", nil
	case "UPDATE", "DELETE":
		if !hasTopLevelWhere(upper) {
			return "write", "high", []string{
				fmt.Sprintf("%s affects every row in the table (no WHERE clause)", first),
			}
		}
		return "write", "medium", nil
	case "TRUNCATE":
		return "destructive", "critical", []string{"TRUNCATE removes all rows and is typically non-rollbackable"}
	case "DROP":
		return "destructive", "critical", []string{"DROP permanently removes " + dropTarget(upper) + " and its data"}
	case "ALTER":
		return analyzeAlter(upper)
	case "CREATE", "REPLACE", "COMMENT", "GRANT", "REVOKE", "VACUUM", "ANALYZE", "REINDEX":
		return "ddl", "medium", nil
	default:
		return "write", "high", []string{"Unrecognized statement keyword " + first + " — review manually"}
	}
}

// hasTopLevelWhere reports whether WHERE appears outside any parentheses, so
// a subquery's WHERE does not mask an unbounded outer UPDATE/DELETE.
func hasTopLevelWhere(upperStmt string) bool {
	depth := 0
	for _, w := range strings.Fields(upperStmt) {
		depth += strings.Count(w, "(") - strings.Count(w, ")")
		if w == "WHERE" && depth <= 0 {
			return true
		}
	}
	return false
}

func dropTarget(upper string) string {
	fields := strings.Fields(upper)
	if len(fields) >= 2 {
		return strings.ToLower(fields[1])
	}
	return "an object"
}

var alterTableRe = regexp.MustCompile(`(?i)\bALTER\s+TABLE\s+([A-Za-z_][A-Za-z0-9_$.]*)`)

// alterTypeTargets returns the deduplicated tables whose column-type change
// forces a rewrite. The input must already be literal-stripped.
func alterTypeTargets(stripped string) []string {
	var targets []string
	seen := map[string]bool{}
	for _, stmt := range splitStatements(stripped) {
		upper := strings.ToUpper(stmt)
		isTypeChange := strings.Contains(upper, " TYPE ") || // PostgreSQL: ALTER COLUMN c TYPE t
			strings.Contains(upper, " MODIFY ") || // MySQL: ALTER COLUMN c MODIFY ...
			strings.Contains(upper, " CHANGE ") // MySQL: ALTER COLUMN c CHANGE ...
		if !isTypeChange {
			continue
		}
		m := alterTableRe.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		table := stripSchema(strings.ToLower(m[1]))
		if table == "" || seen[table] {
			continue
		}
		seen[table] = true
		targets = append(targets, table)
	}
	return targets
}

// enrichWithRewriteSizes upgrades the generic "large tables" rewrite advisory
// with the engine's live row estimate. Best-effort: a failed introspection
// leaves the report untouched rather than failing the analysis.
func (uc *DatabaseUseCase) enrichWithRewriteSizes(ctx context.Context, dbID, strippedStatement string, report *RiskReport) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return
	}
	for _, table := range alterTypeTargets(strippedStatement) {
		n, estErr := uc.queryScalar(ctx, dbID, rowEstimateQuery(strings.ToLower(dbType), table))
		if estErr != nil {
			continue
		}
		report.Notes = append(report.Notes, fmt.Sprintf(
			"Table %q holds ~%s rows (engine estimate); the column type change rewrites all of them and can take locks", table, n))
	}
}

// analyzeAlter flags column drops as destructive and column-type changes as
// potential rewrites (engine-dependent, but worth an advisory).
func analyzeAlter(upper string) (string, string, []string) {
	var notes []string
	risk := "medium"
	if strings.Contains(upper, " DROP COLUMN ") {
		risk = "high"
		notes = append(notes, "ALTER ... DROP COLUMN permanently discards data")
	}
	if strings.Contains(upper, " TYPE ") || strings.Contains(upper, " MODIFY ") || strings.Contains(upper, " CHANGE ") {
		risk = worstRisk(risk, "high")
		notes = append(notes, "Column type changes can force a full table rewrite and take locks on large tables")
	}
	if len(notes) == 0 {
		notes = append(notes, "Schema change — verify dependent views/queries")
	}
	return "destructive", risk, notes
}

// AnalyzeStatementRisk statically analyzes a statement or semicolon-separated
// batch. Literals and comments are stripped first so data content cannot skew
// the verdict.
func AnalyzeStatementRisk(statement string) RiskReport {
	stmts := splitStatements(stripSQLLiterals(statement))

	report := RiskReport{Risk: "low", Statements: len(stmts)}
	for _, s := range stmts {
		kind, risk, notes := classifySingle(s)
		report.Kind = worstKind(report.Kind, kind)
		report.Risk = worstRisk(report.Risk, risk)
		report.Notes = append(report.Notes, notes...)
		if applicable, missing := singleMissingWhere(s); applicable {
			report.MissingWhere = report.MissingWhere || missing
		}
	}
	if len(stmts) > 1 {
		report.Notes = append(report.Notes, fmt.Sprintf(
			"Batch contains %d statements; they execute sequentially in one call", len(stmts)))
	}
	if report.Kind == "" {
		report.Kind = "read"
	}
	return report
}

func singleMissingWhere(stmt string) (applicable, missing bool) {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	first := upper
	if i := strings.IndexAny(upper, " \t\n("); i >= 0 {
		first = upper[:i]
	}
	if first != "UPDATE" && first != "DELETE" {
		return false, false
	}
	return true, !hasTopLevelWhere(upper)
}

// ExecuteStatementDryRun returns the risk report for a statement without
// executing anything. It works even when the database is unreachable, since
// classification is static; when the database IS reachable, column-type-change
// advisories gain the engine's live row estimate.
func (uc *DatabaseUseCase) ExecuteStatementDryRun(ctx context.Context, dbID, statement string) (*RiskReport, error) {
	report := AnalyzeStatementRisk(statement)
	uc.enrichWithRewriteSizes(ctx, dbID, stripSQLLiterals(statement), &report)
	report.Notes = append(report.Notes, "Dry run: nothing was executed against "+dbID)
	return &report, nil
}
