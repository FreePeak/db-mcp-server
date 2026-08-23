package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTimeoutGuardsProbe proves the probe reads both GUCs and targets
// PostgreSQL only.
func TestTimeoutGuardsProbe(t *testing.T) {
	q := timeoutGuardsProbe("postgres")
	if !strings.Contains(q, "idle_in_transaction_session_timeout") ||
		!strings.Contains(q, "statement_timeout") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if timeoutGuardsProbe("postgresql") == "" {
		t.Fatal("postgresql alias must be supported")
	}
	if timeoutGuardsProbe("mysql") != "" || timeoutGuardsProbe("sqlite") != "" {
		t.Fatal("only postgres exposes these session guards")
	}
}

// TestTimeoutGuardsVerdict proves the escalation ladder: unlimited
// statement_timeout, unlimited idle-in-transaction timeout, both,
// and healthy configurations.
func TestTimeoutGuardsVerdict(t *testing.T) {
	if got := timeoutGuardsVerdict(30000, 60000); got != "" {
		t.Fatalf("healthy config must render empty, got:\n%s", got)
	}
	got := timeoutGuardsVerdict(0, 60000)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "statement_timeout") || !strings.Contains(got, "SET") {
		t.Fatalf("unset statement_timeout not escalated:\n%s", got)
	}
	got = timeoutGuardsVerdict(5000, 0)
	if !strings.Contains(got, "idle_in_transaction_session_timeout") {
		t.Fatalf("unset idle-in-transaction timeout not escalated:\n%s", got)
	}
	got = timeoutGuardsVerdict(0, 0)
	if strings.Count(got, "WARNING") != 2 {
		t.Fatalf("both-unset must produce two warnings:\n%s", got)
	}
}

// TestAuditTimeoutGuards_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditTimeoutGuardsUnsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditTimeoutGuards(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
