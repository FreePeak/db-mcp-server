package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFlushMethodProbe proves the probe targets MySQL/MariaDB only.
func TestFlushMethodProbe(t *testing.T) {
	if q := flushMethodQuery("mysql"); !strings.Contains(q, "innodb_flush_method") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if flushMethodQuery("postgres") != "" || flushMethodQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_flush_method")
	}
}

// TestFlushMethodVerdict proves the escalation ladder.
func TestFlushMethodVerdict(t *testing.T) {
	for _, ok := range []string{"O_DIRECT", "O_DIRECT_NO_FSYNC"} {
		if got := flushMethodVerdict(ok); got != "" {
			t.Fatalf("healthy %q must render empty, got:\n%s", ok, got)
		}
	}
	got := flushMethodVerdict("")
	if !strings.Contains(got, "fsync") || !strings.Contains(got, "O_DIRECT") {
		t.Fatalf("default method not escalated:\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "restart") && !strings.Contains(strings.ToLower(got), "config") {
		t.Fatalf("warning must note config/restart, got:\n%s", got)
	}
}

// TestAuditFlushMethod_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditFlushMethod_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditFlushMethod(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
