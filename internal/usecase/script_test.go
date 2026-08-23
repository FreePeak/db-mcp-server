package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestExecuteScript proves cycle 81: a multi-statement script runs
// atomically — all statements commit together, and any failure rolls back
// everything with the failing index reported.
func TestExecuteScript(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	// Success path: both inserts land.
	out, err := uc.ExecuteScript(context.Background(), "db1",
		"INSERT INTO t (id, v) VALUES (1, 'a'); INSERT INTO t (id, v) VALUES (2, 'b');")
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	for _, want := range []string{"2 statement(s)", "committed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	// Failure path: second insert violates PK; first must roll back.
	_, err = uc.ExecuteScript(context.Background(), "db1",
		"INSERT INTO t (id, v) VALUES (3, 'c'); INSERT INTO t (id, v) VALUES (1, 'dup');")
	if err == nil {
		t.Fatal("PK violation must fail the script")
	}
	if !strings.Contains(err.Error(), "statement 2") {
		t.Fatalf("failing statement not identified:\n%v", err)
	}
	var n int
	if scanErr := raw.QueryRow(`SELECT COUNT(*) FROM t WHERE id = 3`).Scan(&n); scanErr != nil {
		t.Fatalf("verify failed: %v", scanErr)
	}
	if n != 0 {
		t.Fatal("statement 1 was not rolled back after statement 2 failed")
	}
}
