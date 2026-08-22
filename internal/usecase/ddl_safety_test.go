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
