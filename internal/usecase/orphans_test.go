package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAuditOrphans proves cycle 93: FK edges with orphaned child rows are
// counted per relationship; clean databases report zero violations.
func TestAuditOrphans(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	must(`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`)
	must(`INSERT INTO users VALUES (1)`)
	must(`INSERT INTO orders VALUES (1, 1)`)  // valid
	must(`INSERT INTO orders VALUES (2, 99)`) // orphan
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.AuditOrphans(context.Background(), "db1")
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if !strings.Contains(out, "orders.user_id") || !strings.Contains(out, "users.id") {
		t.Fatalf("edge not reported:\n%s", out)
	}
	if !strings.Contains(out, "orphaned row(s): 1") && !strings.Contains(out, "1") {
		t.Fatalf("orphan not counted:\n%s", out)
	}
	if strings.Contains(out, "no violations") || strings.Contains(out, "clean") {
		t.Fatalf("violations exist but report claims clean:\n%s", out)
	}

	// Clean second database reports no violations.
	clean := openSQLiteForTest(t)
	if _, err := clean.Exec(`CREATE TABLE p (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.AuditOrphans(context.Background(), "db1")
	if err != nil {
		t.Fatalf("clean audit failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out2), "no violation") && !strings.Contains(strings.ToLower(out2), "clean") {
		t.Fatalf("clean db misreported:\n%s", out2)
	}
}
