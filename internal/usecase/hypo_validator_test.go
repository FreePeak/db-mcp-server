package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestCandidatesForQuery locks in the validator's proposal set: one
// composite per table (eq first, sort appended, capped at 3 columns) plus
// range/join singles not already led by the composite.
func TestCandidatesForQuery(t *testing.T) {
	advice := extractIndexAdvice(`SELECT * FROM orders o JOIN customers c ON c.id = o.customer_id
WHERE o.tenant_id = 7 AND o.total > 100 ORDER BY o.created_at DESC`)
	cands := candidatesForQuery(advice)

	byTable := map[string][]indexCandidate{}
	for _, c := range cands {
		byTable[c.table] = append(byTable[c.table], c)
	}
	orders := byTable["orders"]
	if len(orders) == 0 {
		t.Fatalf("expected candidates for orders, got %v", cands)
	}
	// Composite must lead with equality column tenant_id and append sort.
	foundComp := false
	for _, c := range orders {
		if len(c.cols) >= 2 && c.cols[0] == "tenant_id" {
			foundComp = true
			if len(c.cols) > 3 {
				t.Errorf("composite exceeds 3-column cap: %v", c.cols)
			}
		}
	}
	if !foundComp {
		t.Errorf("expected tenant_id-led composite for orders, got %v", orders)
	}
	// The range single (total) must exist; it is never folded into composites.
	rangeSingle := false
	for _, c := range orders {
		if len(c.cols) == 1 && c.cols[0] == "total" {
			rangeSingle = true
		}
	}
	if !rangeSingle {
		t.Errorf("expected single-column candidate for range predicate on total, got %v", orders)
	}

	// Join keys surface as singles on both sides.
	if got := len(byTable["customers"]); got == 0 {
		t.Errorf("expected join-key candidate for customers, got %v", cands)
	}
}

// TestCandidatesForQuery_CapAndDedup verifies the 3-column cap and that a
// composite leading column suppresses its own single-column twin.
func TestCandidatesForQuery_CapAndDedup(t *testing.T) {
	advice := extractIndexAdvice(`SELECT id FROM t WHERE a = 1 AND b = 2 AND c = 3 ORDER BY d`)
	cands := candidatesForQuery(advice)
	for _, c := range cands {
		if len(c.cols) > 3 {
			t.Fatalf("cap violated: %v", c.cols)
		}
		if len(c.cols) == 1 && c.cols[0] == "a" {
			t.Errorf("composite leader 'a' should suppress single-column twin, got %v", cands)
		}
	}
}

// TestHypoIndexName locks in structural parsing of the standard row format:
// data cells follow the dash rule; footers and headers are skipped.
func TestHypoIndexName(t *testing.T) {
	res := "Results:\n\nindexname\n--------------------------------------------------------------------------------\n<13595>btree_t1_id\nTotal rows: 1"
	if got := hypoIndexName(res); got != "<13595>btree_t1_id" {
		t.Errorf("expected hypo index name from formatted result, got %q", got)
	}
	if got := hypoIndexName("Results:\n\nindexname\n----\n"); got != "" {
		t.Errorf("empty result must yield empty name, got %q", got)
	}
}

// TestValidateIndexSuggestions_NonPostgres verifies the honest refusal for
// engines without a hypopg equivalent — no error, actionable message.
func TestValidateIndexSuggestions_NonPostgres(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (a INTEGER)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.ValidateIndexSuggestions(context.Background(), "db", "SELECT * FROM t WHERE a = 1")
	if err != nil {
		t.Fatalf("must degrade gracefully, got error: %v", err)
	}
	if !strings.Contains(out, "hypopg") || !strings.Contains(out, "PostgreSQL") {
		t.Errorf("expected engine-refusal guidance, got:\n%s", out)
	}
}

// TestValidateIndexSuggestions_EmptyQuery guards input validation.
func TestValidateIndexSuggestions_EmptyQuery(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ValidateIndexSuggestions(context.Background(), "db", "   "); err == nil {
		t.Fatal("empty query must error")
	}
}

// TestCandidatesForQuery_LoneEqualityColumn locks the fix for the
// leader-suppression guard firing when no composite was emitted: a single
// equality filter with no sort must still produce a candidate.
func TestCandidatesForQuery_LoneEqualityColumn(t *testing.T) {
	advice := extractIndexAdvice(`SELECT * FROM hypo46 WHERE tenant_id = 3`)
	cands := candidatesForQuery(advice)
	if len(cands) != 1 || cands[0].table != "hypo46" || len(cands[0].cols) != 1 || cands[0].cols[0] != "tenant_id" {
		t.Fatalf("expected single candidate hypo46(tenant_id), got %v", cands)
	}
}
