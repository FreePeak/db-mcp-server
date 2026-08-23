package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestExecuteStatement_RiskWarningOnCritical proves destructive statements
// that DO execute carry an explicit risk advisory in their result.
func TestExecuteStatement_RiskWarningOnCritical(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", "DROP TABLE users", nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") || !strings.Contains(strings.ToLower(out), "critical") {
		t.Fatalf("expected risk advisory on destructive execution:\n%s", out)
	}
}

// TestExecuteStatement_NoWarningForBenign proves low/medium statements stay
// clean — no advisory noise for ordinary writes.
func TestExecuteStatement_NoWarningForBenign(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, msg TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", `INSERT INTO logs (msg) VALUES ('hello')`, nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if strings.Contains(out, "Risk notice") {
		t.Fatalf("medium-risk INSERT must not warn:\n%s", out)
	}
}

// TestExecuteStatement_WarningOnMissingWhere proves unbounded UPDATE/DELETE
// warns after execution.
func TestExecuteStatement_WarningOnMissingWhere(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER, flag INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t VALUES (1, 0), (2, 0)`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.ExecuteStatement(context.Background(), "db1", "UPDATE t SET flag = 1", nil)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !strings.Contains(out, "Risk notice") || !strings.Contains(out, "no WHERE") {
		t.Fatalf("expected missing-WHERE advisory:\n%s", out)
	}
}
