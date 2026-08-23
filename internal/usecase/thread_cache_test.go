package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestThreadCacheProbe proves the probe joins the setting to its
// churn counters and targets MySQL only.
func TestThreadCacheProbe(t *testing.T) {
	q := threadCacheProbe("mysql")
	if !strings.Contains(q, "thread_cache_size") || !strings.Contains(q, "Threads_created") {
		t.Fatalf("probe must pair setting with Threads_created:\n%s", q)
	}
	if threadCacheProbe("postgres") != "" || threadCacheProbe("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose thread_cache_size")
	}
}

// TestThreadCacheVerdict proves the escalation ladder.
func TestThreadCacheVerdict(t *testing.T) {
	if got := threadCacheVerdict(16, 10000, 12); got != "" {
		t.Fatalf("low churn must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := threadCacheVerdict(9, 10000, 800)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "Threads_created=800") {
		t.Fatalf("churn not escalated with counts:\n%s", got)
	}
	if !strings.Contains(got, "SET GLOBAL thread_cache_size=") {
		t.Fatalf("verdict must name the fix, got:\n%s", got)
	}
	if got := threadCacheVerdict(8, 0, 0); !strings.Contains(got, "No connections yet") {
		t.Fatalf("pre-traffic misjudged:\n%s", got)
	}
}

// TestAuditThreadCache_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditThreadCache_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditThreadCache(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
