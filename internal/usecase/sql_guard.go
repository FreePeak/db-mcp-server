package usecase

import (
	"regexp"
	"strings"
)

// readOnlyLeadingKeywords are statement-leading verbs that never mutate data
// or schema on their own. Anything else is default-denied by the read-only
// guard so unrecognized statements fail closed.
var readOnlyLeadingKeywords = map[string]bool{
	"SELECT":   true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
	"PRAGMA":   true,
	"WITH":     true, // data-modifying CTEs are still caught by the scan below
}

// scanWriteKeywords are data/schema-mutating verbs. Any occurrence outside of
// string literals classifies the whole statement as a write, which catches
// data-modifying CTEs ("WITH x AS (DELETE ...) SELECT ...") and stacked
// statements ("SELECT 1; DROP TABLE users").
var scanWriteKeywords = map[string]bool{
	"INSERT":   true,
	"UPDATE":   true,
	"DELETE":   true,
	"MERGE":    true,
	"UPSERT":   true,
	"TRUNCATE": true,
	"DROP":     true,
	"ALTER":    true,
	"CREATE":   true,
	"GRANT":    true,
	"REVOKE":   true,
	"VACUUM":   true,
	"REINDEX":  true,
}

// leadingOnlyWriteKeywords mutate data but double as ordinary function names
// inside reads (e.g. REPLACE(name,'a','b') in SQLite/MySQL), so they are only
// treated as writes when they appear as the first keyword of a statement.
var leadingOnlyWriteKeywords = map[string]bool{
	"REPLACE": true,
	"CALL":    true,
	"EXEC":    true,
	"EXECUTE": true,
}

var sqlWordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
var dollarTagRe = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*\$`)

// IsWriteStatement reports whether a SQL statement mutates data or schema.
// It strips comments and string literals before classification so keywords
// inside them cannot evade or trigger the check, then applies an allowlist of
// read-only leading verbs plus a conservative scan for mutating keywords.
//
// This is a best-effort textual guardrail, not a sandbox: pair it with the
// engine's own enforcement and always connect with a least-privilege user.
func IsWriteStatement(query string) bool {
	tokens := sqlWordTokens(stripSQLLiterals(query))
	if len(tokens) == 0 {
		return false
	}

	first := strings.ToUpper(tokens[0])
	if scanWriteKeywords[first] || leadingOnlyWriteKeywords[first] {
		return true
	}
	if !readOnlyLeadingKeywords[first] {
		// Default deny: SET, USE, LOCK, LISTEN, unknown verbs, ... are all
		// treated as potential writes on a read-only database.
		return true
	}
	for _, tok := range tokens[1:] {
		if scanWriteKeywords[strings.ToUpper(tok)] {
			return true
		}
	}
	return false
}

func sqlWordTokens(s string) []string { return sqlWordRe.FindAllString(s, -1) }

// stripSQLLiterals removes comments and quoted literals/identifiers from a
// SQL string so keyword classification sees only structural tokens:
//   - line comments: -- ...\n
//   - block comments: /* ... */ (non-nested)
//   - single-quoted strings with ” escaping
//   - double-quoted identifiers with "" escaping
//   - backtick identifiers with “ escaping
//   - dollar-quoted strings: $$...$$ and $tag$...$tag$ (PostgreSQL)
func stripSQLLiterals(query string) string {
	var b strings.Builder
	b.Grow(len(query))

	i := 0
	n := len(query)
	for i < n {
		c := query[i]
		switch {
		case c == '-' && i+1 < n && query[i+1] == '-':
			for i < n && query[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && query[i+1] == '*':
			i += 2
			for i+1 < n && !(query[i] == '*' && query[i+1] == '/') {
				i++
			}
			i = min(i+2, n)
		case c == '\'':
			i = skipQuoted(query, i, '\'', '\'')
			b.WriteByte(' ')
		case c == '"':
			i = skipQuoted(query, i, '"', '"')
			b.WriteByte(' ')
		case c == '`':
			i = skipQuoted(query, i, '`', '`')
			b.WriteByte(' ')
		case c == '$':
			if tag := dollarTagRe.FindString(query[i:]); tag != "" {
				i = skipDollarQuoted(query, i, tag)
				b.WriteByte(' ')
			} else if i+1 < n && query[i+1] == '$' {
				i = skipDollarQuoted(query, i, "$$")
				b.WriteByte(' ')
			} else {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// skipQuoted returns the index just past a quoted run starting at pos
// (query[pos] == open). Doubled delimiters are treated as escapes.
func skipQuoted(query string, pos int, open, close byte) int {
	i := pos + 1
	n := len(query)
	for i < n {
		if query[i] == close {
			if i+1 < n && query[i+1] == close {
				i += 2 // escaped delimiter ('' or "" or ``)
				continue
			}
			return i + 1
		}
		i++
	}
	return n
}

// skipDollarQuoted returns the index just past a PostgreSQL dollar-quoted
// string starting at pos, where tag is the opening delimiter ($$ or $tag$).
func skipDollarQuoted(query string, pos int, tag string) int {
	rest := strings.Index(query[pos+len(tag):], tag)
	if rest < 0 {
		return len(query)
	}
	return pos + len(tag) + rest + len(tag)
}
