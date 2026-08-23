package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestTimeoutGuardrailsCatalog proves per-engine guardrail SELECTs
// cover the runaway-query settings.
func TestTimeoutGuardrailsCatalog(t *testing.T) {
	pg := guardrailsQuery("postgres")
	if !strings.Contains(pg, "statement_timeout") ||
		!strings.Contains(pg, "idle_in_transaction_session_timeout") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := guardrailsQuery("mysql")
	if !strings.Contains(my, "max_execution_time") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if guardrailsQuery("sqlite") != "" {
		t.Fatal("sqlite should have no guardrails catalog")
	}
}

// TestCheckTimeoutGuardrails_Unsupported proves unsupported engines get
// an explicit error.
func TestCheckTimeoutGuardrails_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckTimeoutGuardrails(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
