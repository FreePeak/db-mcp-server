package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAnalyzeStatementRisk_Classification locks the kind/risk verdict for
// every statement class the execute tool accepts.
func TestAnalyzeStatementRisk_Classification(t *testing.T) {
	tests := []struct {
		name      string
		stmt      string
		wantKind  string
		wantLevel string
	}{
		{"select", "SELECT * FROM users", "read", "low"},
		{"insert", "INSERT INTO logs VALUES (1)", "write", "medium"},
		{"update_no_where", "UPDATE users SET active = true", "write", "high"},
		{"update_where", "UPDATE users SET active = true WHERE id = 5", "write", "medium"},
		{"delete_no_where", "DELETE FROM sessions", "write", "high"},
		{"truncate", "TRUNCATE TABLE events", "destructive", "critical"},
		{"drop_table", "DROP TABLE legacy_data", "destructive", "critical"},
		{"drop_database", "DROP DATABASE app_prod", "destructive", "critical"},
		{"alter_drop_column", "ALTER TABLE users DROP COLUMN legacy_flag", "destructive", "high"},
		{"create_table", "CREATE TABLE t (id INT)", "ddl", "medium"},
		{"create_index", "CREATE INDEX idx ON users (email)", "ddl", "medium"},
		{"unknown_verb", "LOCK TABLE users IN EXCLUSIVE MODE", "write", "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AnalyzeStatementRisk(tt.stmt)
			if r.Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q (%+v)", r.Kind, tt.wantKind, r)
			}
			if strings.ToLower(r.Risk) != tt.wantLevel {
				t.Errorf("risk = %q, want %q (%+v)", r.Risk, tt.wantLevel, r)
			}
		})
	}
}

func TestAnalyzeStatementRisk_Flags(t *testing.T) {
	t.Run("missing_where_flagged", func(t *testing.T) {
		r := AnalyzeStatementRisk("UPDATE users SET active = true")
		if !r.MissingWhere {
			t.Fatal("expected MissingWhere=true")
		}
		if len(r.Notes) == 0 {
			t.Fatal("expected an actionable advisory")
		}
	})

	t.Run("where_present_not_flagged", func(t *testing.T) {
		r := AnalyzeStatementRisk("DELETE FROM sessions WHERE expires_at < '2026-01-01'")
		if r.MissingWhere {
			t.Fatal("WHERE present; MissingWhere must be false")
		}
	})

	t.Run("subquery_where_is_not_top_level", func(t *testing.T) {
		// The inner WHERE must not excuse an unbounded outer DELETE.
		r := AnalyzeStatementRisk("DELETE FROM sessions WHERE id IN (SELECT id FROM keep)")
		if r.MissingWhere {
			t.Fatal("a top-level WHERE is present; MissingWhere must be false")
		}
		r = AnalyzeStatementRisk("DELETE FROM sessions WHERE id IN (SELECT id FROM keep WHERE x = 1)")
		if r.MissingWhere {
			t.Fatal("top-level WHERE still present")
		}
		r = AnalyzeStatementRisk("DELETE FROM sessions")
		if !r.MissingWhere {
			t.Fatal("no WHERE at all must be flagged")
		}
	})

	t.Run("comments_and_strings_ignored", func(t *testing.T) {
		// "DROP TABLE" inside a literal or comment must not classify as a write.
		r := AnalyzeStatementRisk("SELECT 'DROP TABLE users' AS note FROM t -- DROP TABLE x")
		if r.Kind != "read" {
			t.Fatalf("string/comment content misclassified as %q (%+v)", r.Kind, r)
		}
	})

	t.Run("stacked_statements_worst_wins", func(t *testing.T) {
		r := AnalyzeStatementRisk("DELETE FROM logs WHERE id = 1; DROP TABLE audits")
		if strings.ToLower(r.Risk) != "critical" {
			t.Fatalf("stacked critical must dominate: %+v", r)
		}
		if !strings.Contains(strings.Join(r.Notes, " "), "2") {
			t.Fatalf("expected a note about the batch size: %+v", r.Notes)
		}
	})

	t.Run("alter_rewrite_warning", func(t *testing.T) {
		r := AnalyzeStatementRisk("ALTER TABLE events ALTER COLUMN payload TYPE text")
		joined := strings.Join(r.Notes, " ")
		if !strings.Contains(joined, "rewrite") && !strings.Contains(joined, "lock") {
			t.Fatalf("expected rewrite/lock advisory: %+v", r.Notes)
		}
	})
}

// TestAlterTypeTargets extracts the tables a column-type change would force
// into a full rewrite; non-rewrite ALTERs must yield nothing.
func TestAlterTypeTargets(t *testing.T) {
	tests := []struct {
		name  string
		stmts string
		want  []string
	}{
		{"postgres_type", "ALTER TABLE users ALTER COLUMN age TYPE bigint", []string{"users"}},
		{"mysql_modify", "ALTER TABLE users MODIFY COLUMN name VARCHAR(200)", []string{"users"}},
		{"mysql_change", "ALTER TABLE users CHANGE COLUMN old_name new_name INT", []string{"users"}},
		{"add_column_no_rewrite", "ALTER TABLE users ADD COLUMN x INT", nil},
		{"drop_column_no_target", "ALTER TABLE users DROP COLUMN x", nil},
		{"schema_qualified", "ALTER TABLE public.users ALTER COLUMN age TYPE bigint", []string{"users"}},
		{"batch_mixed", "ALTER TABLE a ADD COLUMN x INT; ALTER TABLE b MODIFY c INT", []string{"b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alterTypeTargets(stripSQLLiterals(tt.stmts))
			if len(got) != len(tt.want) {
				t.Fatalf("alterTypeTargets(%q) = %v, want %v", tt.stmts, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("alterTypeTargets(%q) = %v, want %v", tt.stmts, got, tt.want)
				}
			}
		})
	}
}

// TestExecuteStatementDryRun proves dry_run reports without touching the
// database, and that the report carries the live row estimate for a table a
// type change would rewrite.
func TestExecuteStatementDryRun(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 0; i < 42; i++ {
		if _, err := raw.Exec(`INSERT INTO items (name) VALUES (?)`, "x"); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	ctx := context.Background()

	report, err := uc.ExecuteStatementDryRun(ctx, "db1", "DROP TABLE users")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if strings.ToLower(report.Risk) != "critical" {
		t.Fatalf("expected critical for DROP TABLE: %+v", report)
	}
	if report.Statements != 1 {
		t.Fatalf("expected 1 statement, got %d", report.Statements)
	}
	if !strings.Contains(strings.Join(report.Notes, " "), "nothing was executed") {
		t.Fatalf("dry run must say nothing was executed: %+v", report.Notes)
	}

	// The table still exists — nothing ran.
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='items'`).Scan(&n); err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if n != 1 {
		t.Fatal("dry run must not touch the database")
	}

	// Rewrite-size enrichment for a type change.
	report, err = uc.ExecuteStatementDryRun(ctx, "db1", "ALTER TABLE items ALTER COLUMN name TYPE text")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	joined := strings.Join(report.Notes, " ")
	if !strings.Contains(joined, "items") || !strings.Contains(joined, "42") {
		t.Fatalf("expected a rewrite-size note naming items with ~42 rows: %v", report.Notes)
	}

	// A non-rewrite ALTER gets no size note.
	report, err = uc.ExecuteStatementDryRun(ctx, "db1", "ALTER TABLE items ADD COLUMN flag INT")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	for _, n := range report.Notes {
		if strings.Contains(n, "rows (engine estimate)") {
			t.Fatalf("ADD COLUMN must not get a rewrite-size note: %v", report.Notes)
		}
	}
}

// TestExecuteStatementDryRun_NoConnectionNeeded: classification is static, so
// a dry run must work against a database that cannot be reached.
func TestExecuteStatementDryRun_NoConnectionNeeded(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: nil}, dbType: "sqlite"})

	report, err := uc.ExecuteStatementDryRun(context.Background(), "offline", "TRUNCATE TABLE events")
	if err != nil {
		t.Fatalf("dry run must not need a live connection: %v", err)
	}
	if strings.ToLower(report.Risk) != "critical" {
		t.Fatalf("expected critical for TRUNCATE: %+v", report)
	}
}

// TestPostExecutionRiskNotice_RewriteSize: the post-execution advisory carries
// the engine's live row estimate for tables a type change rewrote, not just
// the static wording.
func TestPostExecutionRiskNotice_RewriteSize(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := raw.Exec(`INSERT INTO items (name) VALUES (?)`, "x"); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	notice := uc.postExecutionRiskNotice(context.Background(), "db1", "ALTER TABLE items ALTER COLUMN name TYPE text")
	if !strings.Contains(notice, "~7 rows") || !strings.Contains(notice, "items") {
		t.Fatalf("expected a rewrite-size note in the post-execution notice, got:\n%s", notice)
	}
}

// TestExecuteStatement_RiskAdvisory proves a destructive statement that DID
// execute carries an explicit advisory in its result, so the transcript shows
// what happened rather than only "Rows affected: N".
func TestExecuteStatement_RiskAdvisory(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", "DROP TABLE users", nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") || !strings.Contains(strings.ToLower(out), "critical") {
		t.Fatalf("expected a critical advisory on a destructive execution:\n%s", out)
	}
}

// TestExecuteStatement_NoAdvisoryForBenign proves ordinary writes stay clean:
// advisory noise on every INSERT would train readers to ignore it.
func TestExecuteStatement_NoAdvisoryForBenign(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, msg TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", `INSERT INTO logs (msg) VALUES ('hello')`, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("a medium-risk INSERT must not carry an advisory:\n%s", out)
	}
}

// TestExecuteStatement_AdvisoryOnMissingWhere proves an unbounded UPDATE
// warns after execution.
func TestExecuteStatement_AdvisoryOnMissingWhere(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER, flag INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES (1, 0), (2, 0)`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", "UPDATE t SET flag = 1", nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") || !strings.Contains(out, "no WHERE") {
		t.Fatalf("expected a missing-WHERE advisory:\n%s", out)
	}
}

// TestRiskThreshold_ConfigurableWarnLevel: operators can lower the threshold
// to surface medium writes, or raise it to only critical.
func TestRiskThreshold_ConfigurableWarnLevel(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, msg TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	stmt := `INSERT INTO logs (msg) VALUES ('x')` // medium risk

	out, err := uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("the default (high) threshold must not warn on medium:\n%s", out)
	}

	uc.SetRiskWarnAt("medium")
	out, err = uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") {
		t.Fatalf("a medium threshold must warn on INSERT:\n%s", out)
	}

	uc.SetRiskWarnAt("critical")
	out, err = uc.ExecuteStatement(context.Background(), "db1", stmt, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("a critical threshold must not warn on medium:\n%s", out)
	}

	// An invalid level must fall back to the default, not disable advisories.
	uc.SetRiskWarnAt("nonsense")
	out, err = uc.ExecuteStatement(context.Background(), "db1", "DROP TABLE logs", nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") {
		t.Fatalf("an invalid threshold must fall back to high:\n%s", out)
	}
}
