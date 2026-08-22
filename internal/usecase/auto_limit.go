package usecase

import (
	"fmt"
	"strings"
)

// Auto-LIMIT injection bounds server-side work for unbounded SELECTs.
// max_rows truncates results client-side after the engine already produced
// them; injecting LIMIT lets the engine stop early. Conservative rules:
//   - only SELECT / WITH statements
//   - only when NO top-level LIMIT exists (a subquery's LIMIT does not bound
//     the outer result set, so its presence does not suppress injection)
//   - Oracle excluded by the caller (ROWNUM/FETCH FIRST syntax differs)

func isSQLSeparator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// hasTopLevelLimit reports whether the statement contains a LIMIT keyword at
// parenthesized depth zero. Literals must be stripped by the caller.
func hasTopLevelLimit(stripped string) bool {
	upper := strings.ToUpper(stripped)
	depth := 0
	for i := 0; i < len(upper); i++ {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(upper[i:], "LIMIT") {
				before := byte(' ')
				if i > 0 {
					before = upper[i-1]
				}
				after := byte(' ')
				if i+5 < len(upper) {
					after = upper[i+5]
				}
				if isSQLSeparator(before) && isSQLSeparator(after) {
					return true
				}
			}
		}
	}
	return false
}

// applyAutoLimit appends LIMIT n to an unbounded SELECT/WITH statement.
// Non-SELECT statements, zero limits, and queries that already carry a
// top-level LIMIT are returned unchanged.
func applyAutoLimit(query string, limit int) string {
	if limit <= 0 {
		return query
	}
	trimmed := strings.TrimSpace(query)
	trimmed = strings.TrimSuffix(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return query
	}

	upper := strings.ToUpper(trimmed)
	first := upper
	if i := strings.IndexAny(upper, " \t\n("); i >= 0 {
		first = upper[:i]
	}
	if first != "SELECT" && first != "WITH" {
		return query
	}

	stripped := stripSQLLiterals(trimmed)
	if hasTopLevelLimit(stripped) {
		return query
	}
	return fmt.Sprintf("%s LIMIT %d", trimmed, limit)
}
