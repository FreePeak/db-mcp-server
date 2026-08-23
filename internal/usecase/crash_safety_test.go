package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestCrashSafetyProbe proves the probe reads both PG settings.
func TestCrashSafetyProbe(t *testing.T) {
	q := crashSafetyQuery("postgres")
	if !strings.Contains(q, "fsync") || !strings.Contains(q, "full_page_writes") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if crashSafetyQuery("mysql") != "" || crashSafetyQuery("sqlite") != "" {
		t.Fatal("only postgres exposes fsync/full_page_writes")
	}
}

// TestCrashSafetyVerdict proves the escalation ladder.
func TestCrashSafetyVerdict(t *testing.T) {
	if got := crashSafetyVerdict(true, true); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := crashSafetyVerdict(false, true)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "lost") || !strings.Contains(got, "ALTER SYSTEM") {
		t.Fatalf("fsync off not escalated:\n%s", got)
	}
	got = crashSafetyVerdict(true, false)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "torn") {
		t.Fatalf("full_page_writes off not escalated:\n%s", got)
	}
}

// TestAuditCrashSafety_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditCrashSafety_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditCrashSafety(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
