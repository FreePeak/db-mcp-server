package usecase

import (
	"strconv"
	"strings"
)

// rowBoundingWords are top-level keywords that already bound a result set or
// lock its rows, so a server-side cap must not be added on top of them.
var rowBoundingWords = map[string]bool{
	"LIMIT": true, "OFFSET": true, "FETCH": true, "FOR": true, "INTO": true, "LOCK": true,
}

// rowCappedQuery returns query with a server-side row cap of limit rows, so
// the database stops producing rows past the cap; limit+1 rows lets the caller
// detect truncation. LIMIT is appended for non-Oracle engines. Oracle is never
// rewritten: wrapping a SELECT in an inline view raises ORA-00918 when the select
// list repeats a column name, and FETCH FIRST needs 12c+ while the README lists
// 10g+, so client-side truncation still applies. The query is returned unchanged
// when limit <= 0, when it is not a single SELECT or WITH statement, when it is
// a data-modifying statement, or when it already bounds or locks its rows.
func rowCappedQuery(dbType, query string, limit int) string {
	if strings.EqualFold(dbType, "oracle") || limit <= 0 {
		return query
	}
	body := strings.TrimRight(strings.TrimSpace(query), "; \t\r\n")
	if body == "" {
		return query
	}
	stripped := stripSQLLiterals(body)
	switch leadingKeyword(strings.ToUpper(strings.TrimSpace(stripped))) {
	case "SELECT", "WITH":
	default:
		return query
	}
	if IsWriteStatement(query) || strings.Contains(stripped, ";") {
		return query
	}
	for _, w := range topLevelWords(stripped) {
		if rowBoundingWords[w] {
			return query
		}
	}
	return body + "\nLIMIT " + strconv.Itoa(limit)
}

// topLevelWords returns the upper-cased words of a literal-stripped statement
// that sit outside parentheses. A word is a run of letters, digits, '_' or '$'.
func topLevelWords(stripped string) []string {
	var words []string
	depth := 0
	for i := 0; i < len(stripped); {
		c := stripped[i]
		switch {
		case c == '(':
			depth++
			i++
		case c == ')':
			depth = max(depth-1, 0)
			i++
		case isWordByte(c):
			j := i
			for j < len(stripped) && isWordByte(stripped[j]) {
				j++
			}
			if depth == 0 {
				words = append(words, strings.ToUpper(stripped[i:j]))
			}
			i = j
		default:
			i++
		}
	}
	return words
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
