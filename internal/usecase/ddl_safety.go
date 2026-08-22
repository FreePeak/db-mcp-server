package usecase

import (
	"context"
	"fmt"
	"strings"
)

// Statement risk analysis: offline pre-flight checks for the execute tool.
// dry_run reports what a statement WOULD do — kind, risk level, actionable
// notes — without touching the database. This is engine-agnostic static
// analysis at the same classifier layer as read-only enforcement, so it
// works uniformly across PostgreSQL, MySQL, SQLite, and Oracle.

// RiskReport is the structured verdict for one statement (or batch).
type RiskReport struct {
	Kind         string   `json:"kind"`          // read | write | ddl | destructive
	Risk         string   `json:"risk"`          // low | medium | high | critical
	MissingWhere bool     `json:"missing_where"` // UPDATE/DELETE without WHERE
	WouldExecute bool     `json:"would_execute"` // false only in dry-run mode
	Executed     bool     `json:"executed"`      // always false in dry runs
	Statements   int      `json:"statements"`    // statements in the batch
	Notes        []string `json:"notes"`
}

var riskOrder = map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

func worstRisk(a, b string) string {
	if riskOrder[strings.ToLower(a)] >= riskOrder[strings.ToLower(b)] {
		return a
	}
	return b
}

// splitStatements splits on top-level semicolons (literals stripped first).
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
		risk := "medium"
		var notes []string
		if !hasTopLevelWhere(upper) {
			risk = "high"
			notes = append(notes, fmt.Sprintf("%s affects every row in the table (no WHERE clause)", first))
		}
		return "write", risk, notes
	case "TRUNCATE":
		return "destructive", "critical", []string{"TRUNCATE removes all rows and is typically non-rollbackable"}
	case "DROP":
		target := dropTarget(upper)
		return "destructive", "critical", []string{"DROP permanently removes " + target + " and its data"}
	case "ALTER":
		return analyzeAlter(upper)
	case "CREATE", "REPLACE", "COMMENT", "GRANT", "REVOKE", "VACUUM", "ANALYZE":
		return "ddl", "medium", nil
	default:
		return "write", "high", []string{"Unrecognized statement keyword " + first + " — review manually"}
	}
}

func hasTopLevelWhere(upperStmt string) bool {
	depth := 0
	words := strings.Fields(upperStmt)
	for _, w := range words {
		depth += strings.Count(w, "(") - strings.Count(w, ")")
		if w == "WHERE" && depth <= 0 {
			return true
		}
	}
	return false
}

// dropTarget extracts the object type being dropped (table/database/index...).
func dropTarget(upper string) string {
	fields := strings.Fields(upper)
	if len(fields) >= 2 {
		return strings.ToLower(fields[1])
	}
	return "an object"
}

// analyzeAlter flags column drops as destructive and column-type changes as
// potential table rewrites (engine-dependent but worth an advisory).
func analyzeAlter(upper string) (string, string, []string) {
	var notes []string
	risk := "medium"
	isDrop := strings.Contains(upper, " DROP COLUMN ")
	isTypeChange := strings.Contains(upper, " TYPE ") || // PostgreSQL: ALTER COLUMN c TYPE t
		strings.Contains(upper, " MODIFY ") || // MySQL: ALTER COLUMN c MODIFy ...
		strings.Contains(upper, " CHANGE ") // MySQL: ALTER COLUMN c CHANGE ...
	if isDrop {
		risk = "high"
		notes = append(notes, "ALTER ... DROP COLUMN permanently discards data")
	}
	if isTypeChange {
		notes = append(notes, "Column type changes can force a full table rewrite and take locks on large tables")
		risk = worstRisk(risk, "high")
	}
	if len(notes) == 0 {
		notes = append(notes, "Schema change — verify dependent views/queries")
	}
	return "destructive", risk, notes
}

// AnalyzeStatementRisk statically analyzes a statement or semicolon-separated
// batch. Literals and comments are stripped before classification so data
// content cannot skew the verdict.
func AnalyzeStatementRisk(statement string) RiskReport {
	stripped := stripSQLLiterals(statement)
	stmts := splitStatements(stripped)

	report := RiskReport{Risk: "low", Statements: len(stmts)}
	for _, s := range stmts {
		kind, risk, notes := classifySingle(s)
		report.Kind = worstKind(report.Kind, kind)
		report.Risk = worstRisk(report.Risk, risk)
		for _, n := range notes {
			report.Notes = append(report.Notes, n)
		}
		if r, ok := singleMissingWhere(s); ok {
			report.MissingWhere = report.MissingWhere || r
		}
	}
	if len(stmts) > 1 {
		report.Notes = append(report.Notes,
			fmt.Sprintf("Batch contains %d statements; they execute sequentially in one call", len(stmts)))
	}
	if report.Kind == "" {
		report.Kind = "read"
	}
	return report
}

func worstKind(a, b string) string {
	order := map[string]int{"read": 0, "ddl": 1, "write": 2, "destructive": 3}
	if order[b] > order[a] {
		return b
	}
	return a
}

func singleMissingWhere(stmt string) (bool, bool) {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	first := upper
	if i := strings.IndexAny(upper, " \t\n("); i >= 0 {
		first = upper[:i]
	}
	if first != "UPDATE" && first != "DELETE" {
		return false, true // not applicable
	}
	return !hasTopLevelWhere(upper), true
}

// ExecuteStatementDryRun returns the risk report for a statement without
// executing anything. Works even when the database is unreachable because
// no connection is required.
func (uc *DatabaseUseCase) ExecuteStatementDryRun(_ context.Context, dbID, statement string) (*RiskReport, error) {
	report := AnalyzeStatementRisk(statement)
	report.WouldExecute = true
	report.Executed = false
	report.Notes = append(report.Notes, "Dry run: nothing was executed against "+dbID)
	return &report, nil
}
