package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestEffectiveIOConcurrencyProbe proves the probe reads the setting
// and targets PostgreSQL only.
func TestEffectiveIOConcurrencyProbe(t *testing.T) {
	q := effectiveIOConcurrencyProbe("postgres")
	if !strings.Contains(q, "current_setting('effective_io_concurrency')") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if effectiveIOConcurrencyProbe("mysql") != "" || effectiveIOConcurrencyProbe("sqlite") != "" {
		t.Fatal("only postgres exposes effective_io_concurrency")
	}
}

// TestEffectiveIOConcurrencyVerdict proves the escalation ladder.
func TestEffectiveIOConcurrencyVerdict(t *testing.T) {
	if got := effectiveIOConcurrencyVerdict(200); got != "" {
		t.Fatalf("200 must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := effectiveIOConcurrencyVerdict(1) // the default
	if !strings.Contains(got, "spinning disk") || !strings.Contains(got, "prefetch") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	for _, want := range []string{"SSD", "ALTER SYSTEM SET effective_io_concurrency"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verdict missing %q:\n%s", want, got)
		}
	}
	blank := effectiveIOConcurrencyVerdict(0)
	if !strings.Contains(blank, "unreadable") && !strings.Contains(blank, "disabled") {
		t.Fatalf("zero misjudged:\n%s", blank)
	}
}

// TestAuditEffectiveIOConcurrency_Unsupported proves non-PG engines
// get an explicit error.
func TestAuditEffectiveIOConcurrency_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditEffectiveIOConcurrency(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
