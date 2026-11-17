package usecase

import (
	"strconv"
	"strings"
)

// rowBoundingWords are top-level words after which no LIMIT is appended: they
// already bound or lock the rows, or (VALUES) end a compound arm that SQLite
// rejects a trailing LIMIT after.
var rowBoundingWords = map[string]bool{
	"LIMIT": true, "OFFSET": true, "FETCH": true, "FOR": true, "INTO": true, "LOCK": true,
	"VALUES": true,
}

// rowCappedQuery returns query with a server-side row cap of limit rows, so
// the database stops producing rows past the cap; limit+1 rows lets the caller
// detect truncation. Only PostgreSQL (TimescaleDB connections are postgres),
// MySQL and SQLite are rewritten. Oracle is never rewritten: wrapping a SELECT
// in an inline view raises ORA-00918 when the select list repeats a column name,
// and FETCH FIRST needs 12c+ while the README lists 10g+, so client-side
// truncation still applies. The query is returned unchanged when limit <= 0,
// when it is not a single SELECT or WITH statement, when it is a data-modifying
// statement, when it already bounds or locks its rows, or when it contains a
// backslash, '#' or MySQL '/*!' comment that the literal stripper cannot model.
func rowCappedQuery(dbType, query string, limit int) string {
	if limit <= 0 {
		return query
	}
	switch dbType {
	case "postgres", "mysql", "sqlite":
	default:
		return query
	}
	body := strings.TrimRight(strings.TrimSpace(query), "; \t\r\n")
	if body == "" {
		return query
	}
	// stripSQLLiterals does not model backslash escapes, MySQL '#' comments or
	// MySQL '/*!' executable comments, so a bound hidden behind one of them
	// would go unseen and a second LIMIT would be appended. Leave such SQL
	// unchanged; client-side truncation still applies.
	if strings.ContainsAny(body, `\#`) || strings.Contains(body, "/*!") {
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
