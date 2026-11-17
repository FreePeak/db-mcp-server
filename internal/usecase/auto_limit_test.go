package usecase

import "testing"

func TestRowCappedQuery(t *testing.T) {
	tests := []struct {
		name   string
		dbType string
		query  string
		limit  int
		want   string
	}{
		{name: "appends LIMIT", dbType: "postgres", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t\nLIMIT 11"},
		{name: "limit zero is unchanged", dbType: "postgres", query: "SELECT * FROM t", limit: 0, want: "SELECT * FROM t"},
		{name: "trims whitespace and trailing semicolon", dbType: "postgres", query: "  SELECT * FROM t ;  ", limit: 11, want: "SELECT * FROM t\nLIMIT 11"},
		{name: "where and order by", dbType: "postgres", query: "SELECT * FROM t WHERE a = 1 ORDER BY b", limit: 11, want: "SELECT * FROM t WHERE a = 1 ORDER BY b\nLIMIT 11"},
		{name: "with clause", dbType: "postgres", query: "WITH x AS (SELECT 1) SELECT * FROM x", limit: 11, want: "WITH x AS (SELECT 1) SELECT * FROM x\nLIMIT 11"},
		{name: "trailing line comment", dbType: "postgres", query: "SELECT * FROM t -- note", limit: 11, want: "SELECT * FROM t -- note\nLIMIT 11"},
		{name: "leading line comment", dbType: "postgres", query: "-- top rows\nSELECT * FROM t", limit: 11, want: "-- top rows\nSELECT * FROM t\nLIMIT 11"},
		{name: "leading block comment", dbType: "postgres", query: "/* hint */ SELECT * FROM t", limit: 11, want: "/* hint */ SELECT * FROM t\nLIMIT 11"},
		{name: "existing LIMIT", dbType: "postgres", query: "SELECT * FROM t LIMIT 5", limit: 11, want: "SELECT * FROM t LIMIT 5"},
		{name: "lowercase limit", dbType: "postgres", query: "select * from t limit 5", limit: 11, want: "select * from t limit 5"},
		{name: "OFFSET", dbType: "postgres", query: "SELECT * FROM t OFFSET 5", limit: 11, want: "SELECT * FROM t OFFSET 5"},
		{name: "FETCH FIRST", dbType: "postgres", query: "SELECT * FROM t FETCH FIRST 5 ROWS ONLY", limit: 11, want: "SELECT * FROM t FETCH FIRST 5 ROWS ONLY"},
		{name: "FOR UPDATE", dbType: "postgres", query: "SELECT * FROM t FOR UPDATE", limit: 11, want: "SELECT * FROM t FOR UPDATE"},
		{name: "SELECT INTO", dbType: "postgres", query: "SELECT a INTO b FROM t", limit: 11, want: "SELECT a INTO b FROM t"},
		{name: "LOCK IN SHARE MODE", dbType: "postgres", query: "SELECT * FROM t LOCK IN SHARE MODE", limit: 11, want: "SELECT * FROM t LOCK IN SHARE MODE"},
		{name: "LIMIT inside subquery does not count", dbType: "postgres", query: "SELECT * FROM (SELECT * FROM t LIMIT 5) s", limit: 11, want: "SELECT * FROM (SELECT * FROM t LIMIT 5) s\nLIMIT 11"},
		{name: "LIMIT inside string literal does not count", dbType: "postgres", query: "SELECT 'LIMIT' AS w FROM t", limit: 11, want: "SELECT 'LIMIT' AS w FROM t\nLIMIT 11"},
		{name: "identifier containing a keyword does not count", dbType: "postgres", query: "SELECT limited FROM t", limit: 11, want: "SELECT limited FROM t\nLIMIT 11"},
		{name: "INSERT", dbType: "postgres", query: "INSERT INTO t VALUES (1)", limit: 11, want: "INSERT INTO t VALUES (1)"},
		{name: "UPDATE", dbType: "postgres", query: "UPDATE t SET a = 1", limit: 11, want: "UPDATE t SET a = 1"},
		{name: "DELETE", dbType: "postgres", query: "DELETE FROM t", limit: 11, want: "DELETE FROM t"},
		{name: "CREATE", dbType: "postgres", query: "CREATE TABLE t (a int)", limit: 11, want: "CREATE TABLE t (a int)"},
		{name: "EXPLAIN", dbType: "postgres", query: "EXPLAIN SELECT * FROM t", limit: 11, want: "EXPLAIN SELECT * FROM t"},
		{name: "SHOW", dbType: "postgres", query: "SHOW TABLES", limit: 11, want: "SHOW TABLES"},
		{name: "PRAGMA", dbType: "postgres", query: "PRAGMA table_info(t)", limit: 11, want: "PRAGMA table_info(t)"},
		{name: "VALUES", dbType: "postgres", query: "VALUES (1)", limit: 11, want: "VALUES (1)"},
		{name: "data-modifying CTE", dbType: "postgres", query: "WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d", limit: 11, want: "WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d"},
		{name: "stacked statements", dbType: "postgres", query: "SELECT 1; SELECT 2", limit: 11, want: "SELECT 1; SELECT 2"},
		{name: "semicolon then comment", dbType: "postgres", query: "SELECT 1; -- trailing", limit: 11, want: "SELECT 1; -- trailing"},
		{name: "parenthesised union", dbType: "postgres", query: "(SELECT 1) UNION (SELECT 2)", limit: 11, want: "(SELECT 1) UNION (SELECT 2)"},
		{name: "empty", dbType: "postgres", query: "", limit: 11, want: ""},
		{name: "dialect mysql", dbType: "mysql", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t\nLIMIT 11"},
		{name: "dialect sqlite", dbType: "sqlite", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t\nLIMIT 11"},
		{name: "dialect timescaledb", dbType: "timescaledb", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t\nLIMIT 11"},
		{name: "oracle is never rewritten", dbType: "oracle", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t"},
		{name: "oracle ORDER BY is never rewritten", dbType: "oracle", query: "SELECT * FROM t ORDER BY a", limit: 11, want: "SELECT * FROM t ORDER BY a"},
		{name: "oracle name is case-insensitive", dbType: "Oracle", query: "SELECT * FROM t", limit: 11, want: "SELECT * FROM t"},
		{name: "oracle join with duplicate column names is never rewritten", dbType: "oracle", query: "SELECT a.id, b.id FROM a JOIN b ON a.id = b.id", limit: 11, want: "SELECT a.id, b.id FROM a JOIN b ON a.id = b.id"},
		{name: "oracle trailing comment is never rewritten", dbType: "oracle", query: "SELECT * FROM t -- note", limit: 11, want: "SELECT * FROM t -- note"},
		{name: "oracle ROWNUM bound", dbType: "oracle", query: "SELECT * FROM t WHERE ROWNUM <= 5", limit: 11, want: "SELECT * FROM t WHERE ROWNUM <= 5"},
		{name: "oracle FETCH FIRST bound", dbType: "oracle", query: "SELECT * FROM t FETCH FIRST 5 ROWS ONLY", limit: 11, want: "SELECT * FROM t FETCH FIRST 5 ROWS ONLY"},
		{name: "oracle FOR UPDATE", dbType: "oracle", query: "SELECT * FROM t FOR UPDATE", limit: 11, want: "SELECT * FROM t FOR UPDATE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rowCappedQuery(tc.dbType, tc.query, tc.limit); got != tc.want {
				t.Fatalf("rowCappedQuery(%q, %q, %d)\n got: %q\nwant: %q", tc.dbType, tc.query, tc.limit, got, tc.want)
			}
		})
	}
}
