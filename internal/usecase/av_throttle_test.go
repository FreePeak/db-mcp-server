package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestAVThrottleProbe proves the probe reads both cost settings,
// PostgreSQL only.
func TestAVThrottleProbe(t *testing.T) {
	q := avThrottleQuery("postgres")
	if !strings.Contains(q, "autovacuum_vacuum_cost_delay") ||
		!strings.Contains(q, "autovacuum_vacuum_cost_limit") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if avThrottleQuery("mysql") != "" || avThrottleQuery("sqlite") != "" {
		t.Fatal("only postgres exposes autovacuum cost settings")
	}
}

// TestAVThrottleVerdict proves the escalation ladder.
func TestAVThrottleVerdict(t *testing.T) {
	if got := avThrottleVerdict("0", "2000"); got != "" {
		t.Fatalf("unthrottled must render empty, got:\n%s", got)
	}
	if got := avThrottleVerdict("1ms", "5000"); got != "" {
		t.Fatalf("raised limit must render empty, got:\n%s", got)
	}
	got := avThrottleVerdict("2ms", "-1")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "spinning-disk") {
		t.Fatalf("default budget not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM") && !strings.Contains(got, "reload") {
		t.Fatalf("warning must name the fix path, got:\n%s", got)
	}
	if got := avThrottleVerdict("", ""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditAVThrottle_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditAVThrottle_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditAVThrottle(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
