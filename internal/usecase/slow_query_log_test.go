package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSlowQueryLogProbe proves the probe reads both settings and
// targets MySQL-family engines only.
func TestSlowQueryLogProbe(t *testing.T) {
	q := slowQueryLogProbe("mysql")
	if !strings.Contains(q, "slow_query_log") || !strings.Contains(q, "long_query_time") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if slowQueryLogProbe("mariadb") == "" {
		t.Fatal("mariadb must be supported")
	}
	if slowQueryLogProbe("postgres") != "" || slowQueryLogProbe("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose slow_query_log")
	}
}

// TestSlowQueryLogVerdict proves the escalation ladder: disabled log,
// default 10s threshold, and healthy configurations.
func TestSlowQueryLogVerdict(t *testing.T) {
	if got := slowQueryLogVerdict(true, 1); got != "" {
		t.Fatalf("healthy config must render empty, got:\n%s", got)
	}
	got := slowQueryLogVerdict(false, 10)
	if !strings.Contains(got, "disabled") || !strings.Contains(got, "SET GLOBAL slow_query_log") {
		t.Fatalf("disabled log not escalated:\n%s", got)
	}
	got = slowQueryLogVerdict(true, 10) // the default threshold
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "long_query_time") {
		t.Fatalf("default 10s threshold not escalated:\n%s", got)
	}
}

// TestAuditSlowQueryLog_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditSlowQueryLog_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSlowQueryLog(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
