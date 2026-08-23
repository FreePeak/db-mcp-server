package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSlowLogProbe proves per-engine probes read the right settings.
func TestSlowLogProbe(t *testing.T) {
	q := slowLogQuery("mysql")
	for _, want := range []string{"slow_query_log", "long_query_time"} {
		if !strings.Contains(q, want) {
			t.Fatalf("mysql probe missing %q:\n%s", want, q)
		}
	}
	q = slowLogQuery("postgres")
	if !strings.Contains(q, "log_min_duration_statement") {
		t.Fatalf("postgres probe wrong:\n%s", q)
	}
	if slowLogQuery("sqlite") != "" {
		t.Fatal("sqlite has no server-side slow log")
	}
}

// TestSlowLogVerdict proves the observability escalation.
func TestSlowLogVerdict(t *testing.T) {
	if got := slowLogVerdict("OFF", 2); !strings.Contains(got, "WARNING") || !strings.Contains(got, "slow_query_log=ON") {
		t.Fatalf("disabled log not escalated:\n%s", got)
	}
	if got := slowLogVerdict("ON", 10); !strings.Contains(got, "high") {
		t.Fatalf("loose threshold not flagged:\n%s", got)
	}
	if got := slowLogVerdict("ON", 1); got == "" || strings.Contains(got, "WARNING") || strings.Contains(got, "high") {
		t.Fatalf("healthy config misjudged:\n%s", got)
	}
}

// TestPgSlowLogVerdict proves the Postgres-side classifier.
func TestPgSlowLogVerdict(t *testing.T) {
	if got := pgSlowLogVerdict("-1"); !strings.Contains(got, "WARNING") || !strings.Contains(got, "log_min_duration_statement") {
		t.Fatalf("disabled logging not escalated:\n%s", got)
	}
	if got := pgSlowLogVerdict("1000"); got == "" || strings.Contains(got, "WARNING") {
		t.Fatalf("healthy config misjudged:\n%s", got)
	}
}

// TestAuditSlowLog_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditSlowLog_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSlowLog(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
