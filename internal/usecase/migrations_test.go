package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunMigrations proves cycle 92: versioned .sql files apply once in
// name order, applied ones are recorded, and a re-run is a clean noop.
func TestRunMigrations(t *testing.T) {
	raw := openSQLiteForTest(t)
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	write("001_create_users.sql", "CREATE TABLE users (id INTEGER PRIMARY KEY)")
	write("002_seed.sql", "INSERT INTO users (id) VALUES (1)")
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.RunMigrations(context.Background(), "db1", dir)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(out, "2") || !strings.Contains(out, "applied") {
		t.Fatalf("expected 2 applied:\n%s", out)
	}

	// Re-run: everything already applied.
	out, err = uc.RunMigrations(context.Background(), "db1", dir)
	if err != nil {
		t.Fatalf("re-run failed: %v", err)
	}
	if !strings.Contains(out, "No pending") {
		t.Fatalf("re-run should be a noop:\n%s", out)
	}
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM _mcp_migrations`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("recorded = %d err=%v, want 2", n, err)
	}
}

// TestRunMigrations_FailureAtomic proves a failing migration rolls back
// fully and is not recorded, while earlier ones stay committed.
func TestRunMigrations_FailureAtomic(t *testing.T) {
	raw := openSQLiteForTest(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "001_ok.sql"), []byte("CREATE TABLE t (id INTEGER PRIMARY KEY)"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_bad.sql"), []byte("INSERT INTO missing_table VALUES (1)"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	_, err := uc.RunMigrations(context.Background(), "db1", dir)
	if err == nil {
		t.Fatal("bad migration must fail the run")
	}
	if !strings.Contains(err.Error(), "002_bad.sql") {
		t.Fatalf("failing file not named:\n%v", err)
	}
	var n int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM _mcp_migrations`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("recorded = %d err=%v, want only 001", n, err)
	}
	if _, err := raw.Exec(`SELECT 1 FROM t`); err != nil {
		t.Fatalf("001 table should exist after partial failure: %v", err)
	}
}
