package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestPIIAudit proves cycle 101: one call merges name-heuristic and
// content-scan findings, deduped per column.
func TestPIIAudit(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, notes TEXT)`)
	must(`INSERT INTO users VALUES (1, 'jane@corp.io', 'reach me at bob@corp.io')`)
	must(`INSERT INTO users VALUES (2, 'sam@corp.io', 'nothing here')`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.AuditPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	lower := strings.ToLower(out)
	for _, want := range []string{"users.email", "users.notes"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	// users.email must appear once even though both detectors flag it.
	if strings.Count(lower, "users.email") != 1 {
		t.Fatalf("users.email not deduped:\n%s", out)
	}

	// Clean database reports no findings.
	clean := openSQLiteForTest(t)
	if _, err := clean.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.AuditPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("clean audit failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out2), "no pii") && !strings.Contains(strings.ToLower(out2), "no findings") {
		t.Fatalf("clean db misreported:\n%s", out2)
	}
}
