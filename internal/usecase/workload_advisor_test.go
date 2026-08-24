package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/dbtools"
)

// TestWorkloadIndexSuggestions_EndToEnd seeds the server's own query history
// (the fallback source on engines without statement-stats catalogs) with
// several statements against unindexed columns and checks that merged
// suggestions report per-statement coverage.
func TestWorkloadIndexSuggestions_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	// :memory: SQLite gives every pooled connection its own empty database;
	// pin to one connection so DDL and tracked queries share it.
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`CREATE TABLE invoices (id INTEGER PRIMARY KEY, customer_id INTEGER, state TEXT)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	ctx := context.Background()

	seed := func(q string) {
		if _, err := dbtools.GetPerformanceAnalyzer().TrackQuery(ctx, q, nil, func() (interface{}, error) {
			rows, err := raw.Query(q)
			if err != nil {
				return nil, err
			}
			// Close before returning: with one pooled connection an open
			// result set would block the advisor's own catalog queries.
			rows.Close()
			return nil, nil
		}); err != nil {
			t.Fatalf("track failed: %v", err)
		}
	}
	// Three distinct statements hit customer_id; one adds a sort on state.
	for i := 0; i < 3; i++ {
		seed(`SELECT id FROM invoices WHERE customer_id = 42`)
	}
	seed(`SELECT id FROM invoices WHERE customer_id = 42 ORDER BY state`)

	out, err := uc.WorkloadIndexSuggestions(ctx, "db", 0)
	if err != nil {
		t.Fatalf("workload_suggestions failed: %v", err)
	}
	if !strings.Contains(out, "CREATE INDEX idx_invoices_customer_id_state ON invoices (customer_id, state)") {
		t.Fatalf("expected composite suggestion from merged workload, got:\n%s", out)
	}
	// The tracker groups identical query texts: three identical selects are
	// one distinct statement, the sorted variant another — hence 2 of 2.
	if !strings.Contains(out, "serves 2 of 2 statement(s)") {
		t.Errorf("expected coverage annotation across distinct statements, got:\n%s", out)
	}
}

// TestWorkloadIndexSuggestions_LimitClamps guards the analysis cap.
func TestWorkloadIndexSuggestions_InputGuards(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	// A limit above the ceiling must not error; clamping is internal.
	if _, err := uc.WorkloadIndexSuggestions(context.Background(), "db", 999); err != nil {
		t.Fatalf("large limit should clamp, not fail: %v", err)
	}
}

// TestExtractIndexAdvice_NoTables keeps the pure extractor honest: DML
// without FROM/JOIN yields nothing rather than bogus candidates.
func TestExtractIndexAdvice_NoTables(t *testing.T) {
	for _, q := range []string{
		`INSERT INTO audit_log (msg) VALUES ('x')`,
		`UPDATE settings SET value = 'a' WHERE key = 'b'`,
	} {
		if advice := extractIndexAdvice(q); len(advice) != 0 {
			t.Errorf("expected no advice for %q, got %v", q, advice)
		}
	}
}
