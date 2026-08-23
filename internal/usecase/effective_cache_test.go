package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestEffectiveCacheProbe proves the probe targets PostgreSQL only.
func TestEffectiveCacheProbe(t *testing.T) {
	if q := effectiveCacheProbe("postgres"); !strings.Contains(q, "effective_cache_size") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if effectiveCacheProbe("mysql") != "" || effectiveCacheProbe("sqlite") != "" {
		t.Fatal("only postgres exposes effective_cache_size")
	}
}

// TestEffectiveCacheVerdict proves the escalation ladder.
func TestEffectiveCacheVerdict(t *testing.T) {
	if got := effectiveCacheVerdict("24GB"); got != "" {
		t.Fatalf("tuned value must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := effectiveCacheVerdict("4GB")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "default") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM SET effective_cache_size=") || !strings.Contains(got, "pg_reload_conf") {
		t.Fatalf("verdict must name the fix path, got:\n%s", got)
	}
	if got := effectiveCacheVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty/unreadable misjudged:\n%s", got)
	}
}

// TestAuditEffectiveCache_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditEffectiveCache_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditEffectiveCache(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
