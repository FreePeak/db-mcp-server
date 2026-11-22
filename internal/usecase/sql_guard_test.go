package usecase

import "testing"

// TestIsWriteStatement locks in the behavior of the SQL guard used to close
// the read-only bypass: the query_* tool historically executed any statement,
// so an agent could run INSERT/UPDATE/DELETE/DDL through it even when the
// database was configured with read_only: true.
func TestIsWriteStatement(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  bool
	}{
		// Plain reads must pass.
		{"plain select", "SELECT * FROM users", false},
		{"lowercase select", "select 1", false},
		{"leading whitespace", "\n\t  SELECT id FROM t WHERE x = 1", false},
		{"show tables", "SHOW TABLES", false},
		{"describe", "DESCRIBE users", false},
		{"desc", "DESC users", false},
		{"explain", "EXPLAIN ANALYZE SELECT * FROM t", false},
		{"pragma", "PRAGMA table_info(users)", false},
		{"parenthesized select", "(SELECT 1)", false},

		// Comments must be ignored before classification.
		{"line comment then read", "-- fetch rows\nSELECT * FROM t", false},
		{"block comment then write", "/* maintenance */ DELETE FROM t", true},
		{"comment hiding write", "/* SELECT */ UPDATE t SET a = 1", true},

		// Obvious writes.
		{"insert", "INSERT INTO t VALUES (1)", true},
		{"update", "UPDATE t SET a = 1", true},
		{"delete", "DELETE FROM t", true},
		{"merge", "MERGE INTO t USING s ON (t.id = s.id) WHEN MATCHED THEN UPDATE SET a = 1", true},
		{"truncate", "TRUNCATE TABLE t", true},
		{"drop", "DROP TABLE users", true},
		{"alter", "ALTER TABLE t ADD COLUMN c INT", true},
		{"create", "CREATE TABLE t (id INT)", true},
		{"grant", "GRANT SELECT ON t TO app", true},
		{"revoke", "REVOKE ALL ON t FROM app", true},
		{"call", "CALL do_maintenance()", true},
		{"oracle exec", "EXEC purge_logs", true},

		// String literals must not leak keywords into the classifier.
		{"write verb inside string literal", "SELECT * FROM t WHERE msg = 'please DELETE me'", false},
		{"escaped quote string", "SELECT * FROM t WHERE msg = 'it''s a DROP day'", false},
		{"double-quoted identifier", `SELECT "delete" FROM t`, false},
		{"backtick identifier", "SELECT `update` FROM t", false},
		{"dollar-quoted string", "SELECT $$DELETE FROM secret$$ AS payload", false},
		{"tagged dollar-quoted string", "SELECT $fn$DELETE FROM secret$fn$ AS payload", false},

		// Data-modifying CTEs and multi-statement payloads are writes.
		{"data-modifying cte", "WITH moved AS (DELETE FROM t RETURNING *) INSERT INTO archive SELECT * FROM moved", true},
		{"multi statement trailing drop", "SELECT 1; DROP TABLE users", true},
		{"multi statement leading insert", "INSERT INTO t VALUES (1); SELECT 1", true},

		// Session-state changes issued through a SELECT flip the read-only
		// default on the pooled connection, so they are writes for this guard.
		{"set_config flips read-only default", "SELECT set_config('default_transaction_read_only','off',false)", true},
		{"set_config flips transaction_read_only", "SELECT set_config('transaction_read_only', 'off', false)", true},
		{"quoted set_config call", `SELECT "set_config"('default_transaction_read_only','off',false)`, true},
		{"quoted set_config call uppercase", `SELECT "SET_CONFIG"('transaction_read_only','off',false)`, true},
		{"quoted set_config call with space", `SELECT "set_config" ('transaction_read_only','off',false)`, true},
		{"schema-qualified quoted set_config call", `SELECT "pg_catalog"."set_config"('default_transaction_read_only','off',false)`, true},
		{"quoted set_config column", `SELECT "set_config" FROM settings`, false},
		{"quoted set_config alias", `SELECT name AS "set_config" FROM settings`, false},

		// SELECT ... INTO creates a table or writes a file or variable.
		{"select into table", "SELECT * INTO newt FROM orders", true},
		{"with select into table", "WITH s AS (SELECT * FROM orders) SELECT * INTO newt FROM s", true},
		{"select into outfile", "SELECT * FROM t INTO OUTFILE 'out.csv'", true},
		{"select into dumpfile", "SELECT 1 INTO DUMPFILE 'dump.bin'", true},
		{"select into user variable", "SELECT id INTO @v FROM t", true},
		{"explain of select into", "EXPLAIN SELECT * INTO newt FROM orders", true},

		// INTO inside literals, comments or identifiers is not a keyword.
		{"into inside string literal", "SELECT 'into' AS x", false},
		{"into inside identifier", "SELECT into_date FROM t", false},
		{"into inside block comment", "SELECT 1 /* INTO newt */", false},
		{"into inside line comment", "-- INTO newt\nSELECT 1", false},

		// COPY is not a read verb, so default deny classifies it as a write.
		{"copy to file", "COPY t TO 'out.csv'", true},
		{"copy from file", "COPY t FROM 'in.csv'", true},
		{"copy query to stdout", "COPY (SELECT 1) TO STDOUT", true},

		// Default deny for anything unrecognized.
		{"unknown verb", "VACUUM FULL t", true},
		{"empty", "", false},
		{"whitespace only", "   \n\t ", false},
		{"comment only", "-- nothing here", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWriteStatement(tc.query)
			if got != tc.want {
				t.Fatalf("IsWriteStatement(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}
