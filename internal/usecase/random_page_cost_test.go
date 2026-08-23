package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestRandomPageCostProbe proves the probe targets PostgreSQL only.
func TestRandomPageCostProbe(t *testing.T) {
	if q := randomPageCostQuery("postgres"); !strings.Contains(q, "random_page_cost") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if randomPageCostQuery("mysql") != "" || randomPageCostQuery("sqlite") != "" {
		t.Fatal("only postgres exposes random_page_cost")
	}
}

// TestRandomPageCostVerdict proves the escalation ladder.
func TestRandomPageCostVerdict(t *testing.T) {
	if got := randomPageCostVerdict(1.1); got != "" {
		t.Fatalf("SSD-tuned must render empty (audit adds the clean line), got:\n%s", got)
	}
	if got := randomPageCostVerdict(2.0); got != "" {
		t.Fatalf("already-lowered values must stay quiet, got:\n%s", got)
	}
	got := randomPageCostVerdict(4.0)
	if !strings.Contains(got, "spinning-disk") || !strings.Contains(got, "1.1") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	if !strings.Contains(got, "storage is SSD") || !strings.Contains(got, "ALTER SYSTEM") {
		t.Fatalf("verdict must be conditional on storage type and name the fix path, got:\n%s", got)
	}
	if got := randomPageCostVerdict(0); !strings.Contains(got, "unreadable") {
		t.Fatalf("zero/unreadable misjudged:\n%s", got)
	}
}

// TestAuditRandomPageCost_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditRandomPageCost_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditRandomPageCost(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
