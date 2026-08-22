package usecase

import (
	"context"
	"strings"
	"testing"
)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := AnalyzeStatementRisk(tt.stmt)
			if r.Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", r.Kind, tt.wantKind)
			}
			if strings.ToLower(r.Risk) != tt.wantLevel {
				t.Errorf("risk = %q, want %q (report: %+v)", r.Risk, tt.wantLevel, r)
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
			t.Fatal("expected actionable notes")
		}
	})

	t.Run("where_present_not_flagged", func(t *testing.T) {
		r := AnalyzeStatementRisk("DELETE FROM sessions WHERE expires_at < '2026-01-01'")
		if r.MissingWhere {
			t.Fatal("WHERE present; MissingWhere must be false")
		}
	})

	t.Run("comments_and_strings_ignored", func(t *testing.T) {
		// The word DROP inside a string/comment must not trigger classification.
		r := AnalyzeStatementRisk("SELECT 'DROP TABLE users' AS note FROM t -- DROP TABLE x\nWHERE id = 1")
		if r.Kind != "read" {
			t.Fatalf("string/comment content misclassified as %q", r.Kind)
		}
	})

	t.Run("stacked_statements_worst_wins", func(t *testing.T) {
		r := AnalyzeStatementRisk("DELETE FROM logs WHERE id = 1; DROP TABLE audits")
		if strings.ToLower(r.Risk) != "critical" {
			t.Fatalf("stacked critical must dominate: %+v", r)
		}
		if !strings.Contains(strings.Join(r.Notes, " "), "2") {
			t.Fatalf("expected note about multiple statements: %+v", r.Notes)
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

// TestExecuteDryRun proves dry_run reports without touching the database —
// even against a database that is not connected/reachable.
func TestExecuteDryRun(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	report, err := uc.ExecuteStatementDryRun(context.Background(), "db1", "DROP TABLE users")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if report.Executed {
		t.Fatal("dry run must not execute")
	}
	if strings.ToLower(report.Risk) != "critical" {
		t.Fatalf("expected critical for DROP TABLE: %+v", report)
	}
	if !report.WouldExecute {
		t.Fatal("WouldExecute should reflect that a real call would run it")
	}
}

// TestAlterTypeTargets covers cycle 55: extracting the tables a column-type
// change would force into a full rewrite. Non-rewrite ALTERs must yield
// nothing.
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

// TestExecuteStatementDryRun_RewriteSizeNote proves the dry-run report is
// enriched with the engine's live row estimate for tables a type change
// would rewrite. Introspection failures must never fail the dry run.
func TestExecuteStatementDryRun_RewriteSizeNote(t *testing.T) {
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

	report, err := uc.ExecuteStatementDryRun(context.Background(), "db1",
		"ALTER TABLE items ALTER COLUMN name TYPE text")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	found := false
	for _, n := range report.Notes {
		if strings.Contains(n, "items") && strings.Contains(n, "42") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rewrite-size note naming items with ~42 rows, got notes: %v", report.Notes)
	}

	// A non-rewrite ALTER gets no size note and no introspection round-trip.
	report, err = uc.ExecuteStatementDryRun(context.Background(), "db1",
		"ALTER TABLE items ADD COLUMN flag INT")
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	for _, n := range report.Notes {
		if strings.Contains(n, "rows (engine estimate)") {
			t.Fatalf("ADD COLUMN must not get a rewrite-size note: %v", report.Notes)
		}
	}
}

// TestPostExecutionRiskNotice_RewriteSize proves cycle 56: the
// post-execution risk notice carries the engine's live row estimate for
// tables a type change rewrote, not just the static wording.
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

	notice := uc.postExecutionRiskNotice(context.Background(), "db1",
		"ALTER TABLE items ALTER COLUMN name TYPE text")
	if !strings.Contains(notice, "~7 rows") || !strings.Contains(notice, "items") {
		t.Fatalf("expected rewrite-size note in post-execution notice, got:\n%s", notice)
	}
}
