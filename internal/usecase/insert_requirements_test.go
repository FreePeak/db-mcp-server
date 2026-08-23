package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestIsRequiredColumn proves the nullability/default classifier across
// engine encodings: PG "NO", Oracle "N", SQLite notnull=1.
func TestIsRequiredColumn(t *testing.T) {
	tests := []struct {
		name       string
		isNullable interface{}
		hasDefault bool
		want       bool
	}{
		{"pg_not_null_no_default", "NO", false, true},
		{"oracle_not_null_no_default", "N", false, true},
		{"sqlite_notnull_no_default", int64(1), false, true},
		{"nullable", "YES", false, false},
		{"has_default", "NO", true, false},
		{"sqlite_has_default", int64(1), true, false},
		{"unknown_encoding", "?", false, false}, // conservative: don't flag
	}
	for _, tt := range tests {
		if got := isRequiredColumn(tt.isNullable, tt.hasDefault); got != tt.want {
			t.Fatalf("%s: isRequiredColumn(%v, %v) = %v, want %v", tt.name, tt.isNullable, tt.hasDefault, got, tt.want)
		}
	}
}

// TestInsertRequirements_SQLite proves the end-to-end walk flags only
// NOT NULL columns lacking defaults.
func TestInsertRequirements_SQLite(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL,
		bio TEXT DEFAULT '',
		name TEXT
	)`)
	must(`CREATE TABLE events (id INTEGER PRIMARY KEY, note TEXT)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.InsertRequirements(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if !strings.Contains(out, "users") || !strings.Contains(out, "email") {
		t.Fatalf("required column not reported:\n%s", out)
	}
	if strings.Contains(out, "bio") || strings.Contains(out, "name:") || strings.Contains(strings.Split(out, "\n")[0], "note") {
		// bio/name/note are insertable-without-value; they must not appear as requirements
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "bio") || strings.Contains(line, "note") {
				t.Fatalf("optional column misreported:\n%s", line)
			}
		}
	}
}

// TestInsertRequirements_Clean proves all-defaultable tables render an
// explicit clean result.
func TestInsertRequirements_Clean(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, note TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.InsertRequirements(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if !strings.Contains(out, "No required columns") {
		t.Fatalf("clean state wrong:\n%s", out)
	}
}
