package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestJITProbe proves the probe targets PostgreSQL only.
func TestJITProbe(t *testing.T) {
	if q := jitQuery("postgres"); !strings.Contains(q, "current_setting('jit')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if jitQuery("mysql") != "" || jitQuery("sqlite") != "" {
		t.Fatal("only postgres has a JIT compiler toggle")
	}
}

// TestJITVerdict proves the escalation ladder.
func TestJITVerdict(t *testing.T) {
	if got := jitVerdict("off"); got != "" {
		t.Fatalf("disabled must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := jitVerdict("on")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "LLVM") {
		t.Fatalf("enabled not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM") || !strings.Contains(got, "reload") {
		t.Fatalf("warning must name the fix path, got:\n%s", got)
	}
	if got := jitVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditJIT_Unsupported proves non-PG engines get an explicit
// error.
func TestAuditJIT_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditJIT(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
