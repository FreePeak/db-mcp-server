package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

const (
	pgPlanCheap = `[{"Plan": {"Node Type": "Result", "Total Cost": 0.01}}]`
	pgPlanHuge  = `[{"Plan": {"Node Type": "Nested Loop", "Total Cost": 12345.6}}]`
	myPlanHuge  = `{"query_block": {"select_id": 1, "cost_info": {"query_cost": "12345.60"}, "table": {"table_name": "t"}}}`
)

func TestParsePlanCost_Postgres(t *testing.T) {
	got, err := parsePlanCost("postgres", pgPlanHuge)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 12345.6 {
		t.Fatalf("postgres cost = %v, want 12345.6", got)
	}
	if _, err := parsePlanCost("postgres", `[{"Plan": {"Node Type": "Result"}}]`); err == nil {
		t.Fatal("plan without Total Cost must fail closed, got nil error")
	}
}

func TestParsePlanCost_MySQL(t *testing.T) {
	got, err := parsePlanCost("mysql", myPlanHuge)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 12345.6 {
		t.Fatalf("mysql cost = %v, want 12345.6", got)
	}
}

// MySQL omits cost_info for a query that reads no table (SELECT 1); such a
// plan costs nothing and must not be denied as unreadable.
func TestParsePlanCost_MySQLNoTablesIsZero(t *testing.T) {
	got, err := parsePlanCost("mysql", `{"query_block": {"select_id": 1, "message": "No tables used"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Fatalf("no-table plan cost = %v, want 0", got)
	}
}

func TestParsePlanCost_MySQLUnionSumsBlocks(t *testing.T) {
	union := `{"query_block": {"union_result": {"using_temporary_table": true, "query_specifications": [
		{"query_block": {"select_id": 1, "cost_info": {"query_cost": "10.00"}}},
		{"query_block": {"select_id": 2, "cost_info": {"query_cost": "20.50"}}}]}}}`
	got, err := parsePlanCost("mysql", union)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 30.5 {
		t.Fatalf("union cost = %v, want 30.5 (sum of both blocks)", got)
	}
}

func TestParsePlanCost_Garbage(t *testing.T) {
	cases := []struct {
		name, dbType, plan string
	}{
		{"empty input", "postgres", ""},
		{"not json", "postgres", "not json"},
		{"empty postgres array", "postgres", `[]`},
		{"mysql without cost_info", "mysql", `{"query_block": {"select_id": 1}}`},
		{"mysql non-numeric cost", "mysql", `{"query_block": {"cost_info": {"query_cost": "abc"}}}`},
		{"engine without parser", "sqlite", pgPlanHuge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parsePlanCost(tc.dbType, tc.plan); err == nil {
				t.Fatalf("expected error, got cost %v", got)
			}
		})
	}
}

func TestCostExplainSQL_UnsupportedEngine(t *testing.T) {
	for _, dbType := range []string{"sqlite", "oracle"} {
		if sql, ok := costExplainSQL(dbType, "SELECT 1"); ok || sql != "" {
			t.Fatalf("costExplainSQL(%q) = (%q, %v), want empty and false", dbType, sql, ok)
		}
	}

	if sql, ok := costExplainSQL("postgres", "SELECT 1"); !ok || sql != "EXPLAIN (FORMAT JSON) SELECT 1" {
		t.Fatalf("postgres = (%q, %v), want EXPLAIN (FORMAT JSON) prefix", sql, ok)
	}
	if sql, ok := costExplainSQL("mysql", "SELECT 1"); !ok || sql != "EXPLAIN FORMAT=JSON SELECT 1" {
		t.Fatalf("mysql = (%q, %v), want EXPLAIN FORMAT=JSON prefix", sql, ok)
	}
}

// costDB is a domain.Database double for the cost gate. It records every SQL
// text sent to it and answers EXPLAIN statements with a canned plan.
type costDB struct {
	limit      float64
	plan       string
	explainErr error
	sent       []string
}

func (c *costDB) Query(_ context.Context, query string, _ ...interface{}) (domain.Rows, error) {
	c.sent = append(c.sent, query)
	if strings.HasPrefix(query, "EXPLAIN") {
		if c.explainErr != nil {
			return nil, c.explainErr
		}
		return &fakeRows{columns: []string{"QUERY PLAN"}, data: [][]interface{}{{c.plan}}}, nil
	}
	return newFakeRows(1), nil
}
func (c *costDB) Exec(_ context.Context, _ string, _ ...interface{}) (domain.Result, error) {
	return fakeResult{}, nil
}
func (c *costDB) Begin(_ context.Context, _ *domain.TxOptions) (domain.Tx, error) { return nil, nil }
func (c *costDB) IsReadOnly() bool                                                { return false }
func (c *costDB) MaxRows() int                                                    { return 0 }
func (c *costDB) MaxQueryCost() float64                                           { return c.limit }

// explainCount returns how many EXPLAIN statements reached the database.
func (c *costDB) explainCount() int {
	n := 0
	for _, q := range c.sent {
		if strings.HasPrefix(q, "EXPLAIN") {
			n++
		}
	}
	return n
}

// realCount returns how many non-EXPLAIN statements reached the database.
func (c *costDB) realCount() int { return len(c.sent) - c.explainCount() }

func newCostUseCase(db *costDB, dbType string) *DatabaseUseCase {
	return NewDatabaseUseCase(&fakeRepo{db: db, dbType: dbType})
}

func TestRunQuery_CostGate_DeniesAboveLimit(t *testing.T) {
	db := &costDB{limit: 1000, plan: pgPlanHuge}
	uc := newCostUseCase(db, "postgres")

	_, err := uc.ExecuteQueryMasked(context.Background(), "pg1", "SELECT * FROM a CROSS JOIN b", nil, false)
	if err == nil {
		t.Fatal("expected cost denial, got nil error")
	}
	if !strings.Contains(err.Error(), "12345.6") || !strings.Contains(err.Error(), "1000") {
		t.Fatalf("error must name estimate and limit, got: %v", err)
	}
	if !strings.Contains(err.Error(), "exceeds max_query_cost") {
		t.Fatalf("error must say the limit was exceeded, got: %v", err)
	}
	if db.realCount() != 0 {
		t.Fatalf("denied query must never reach the engine, sent: %q", db.sent)
	}
}

func TestEffectiveCostLimit(t *testing.T) {
	tests := []struct {
		name                 string
		dbLimit, callerLimit float64
		want                 float64
	}{
		{"none set", 0, 0, 0},
		{"connection only", 0, 500, 500},
		{"caller only", 800, 0, 800},
		{"caller tighter", 1000, 300, 300},
		{"connection tighter", 200, 1000, 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveCostLimit(tc.dbLimit, tc.callerLimit); got != tc.want {
				t.Fatalf("effectiveCostLimit(%v, %v) = %v, want %v", tc.dbLimit, tc.callerLimit, got, tc.want)
			}
		})
	}
}

func TestRunQuery_CostGate_CallerLimitApplies(t *testing.T) {
	db := &costDB{limit: 0, plan: pgPlanHuge}
	uc := newCostUseCase(db, "postgres")
	ctx := WithCaller(context.Background(), Caller{Principal: "agent-a", MaxQueryCost: 1000})

	_, err := uc.ExecuteQueryMasked(ctx, "pg1", "SELECT * FROM a CROSS JOIN b", nil, false)
	if err == nil || !strings.Contains(err.Error(), "exceeds max_query_cost 1000") {
		t.Fatalf("caller grant limit must deny an estimate above it on an unlimited connection, got: %v", err)
	}
	if db.realCount() != 0 {
		t.Fatalf("denied query must never reach the engine, sent: %q", db.sent)
	}
}

func TestRunQuery_CostGate_CallerLimitNoEstimateEngineDenies(t *testing.T) {
	db := &costDB{limit: 0, plan: pgPlanCheap}
	uc := newCostUseCase(db, "sqlite")
	ctx := WithCaller(context.Background(), Caller{Principal: "agent-a", MaxQueryCost: 1000})

	_, err := uc.ExecuteQueryMasked(ctx, "lite1", "SELECT 1", nil, false)
	want := `max_query_cost: engine "sqlite" has no cost estimate on database "lite1"`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("caller limit on an engine without cost estimates must fail closed with %q, got: %v", want, err)
	}
	if db.realCount() != 0 {
		t.Fatalf("denied query must never reach the engine, sent: %q", db.sent)
	}
}

func TestRunQuery_CostGate_AllowsBelow(t *testing.T) {
	db := &costDB{limit: 1000, plan: pgPlanCheap}
	uc := newCostUseCase(db, "postgres")

	if _, err := uc.ExecuteQueryMasked(context.Background(), "pg1", "SELECT 1", nil, false); err != nil {
		t.Fatalf("query under the limit must run, got: %v", err)
	}
	if db.explainCount() != 1 || db.realCount() != 1 {
		t.Fatalf("expected one EXPLAIN and one real query, sent: %q", db.sent)
	}
	if db.sent[0] != "EXPLAIN (FORMAT JSON) SELECT 1" {
		t.Fatalf("EXPLAIN text = %q", db.sent[0])
	}
}

func TestRunQuery_CostGate_ExplainErrorFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		db   *costDB
		want string
	}{
		{"explain error", &costDB{limit: 1000, explainErr: errors.New("boom")}, "boom"},
		{"unparsable plan", &costDB{limit: 1000, plan: "garbage"}, "plan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := newCostUseCase(tc.db, "postgres")
			_, err := uc.ExecuteQueryMasked(context.Background(), "pg1", "SELECT 1", nil, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected denial mentioning %q, got: %v", tc.want, err)
			}
			if tc.db.realCount() != 0 {
				t.Fatalf("unverifiable query must be denied, sent: %q", tc.db.sent)
			}
		})
	}
}

func TestRunQuery_CostGate_SkipsNonSelect(t *testing.T) {
	db := &costDB{limit: 1000, plan: pgPlanHuge}
	uc := newCostUseCase(db, "postgres")

	if _, err := uc.ExecuteQueryMasked(context.Background(), "pg1", "SHOW TABLES", nil, false); err != nil {
		t.Fatalf("SHOW must not be gated, got: %v", err)
	}
	if db.explainCount() != 0 {
		t.Fatalf("SHOW must not trigger EXPLAIN, sent: %q", db.sent)
	}
}

func TestRunQuery_CostGate_RefusesRawExplainAnalyze(t *testing.T) {
	for _, stmt := range []string{"EXPLAIN ANALYZE SELECT 1", "EXPLAIN (ANALYZE, BUFFERS) SELECT 1"} {
		t.Run(stmt, func(t *testing.T) {
			db := &costDB{limit: 1000, plan: pgPlanCheap}
			uc := newCostUseCase(db, "postgres")

			_, err := uc.ExecuteQueryMasked(context.Background(), "pg1", stmt, nil, false)
			if err == nil || !strings.Contains(err.Error(), "explain tool") {
				t.Fatalf("expected refusal pointing at the explain tool, got: %v", err)
			}
			if len(db.sent) != 0 {
				t.Fatalf("raw EXPLAIN ANALYZE must not reach the engine, sent: %q", db.sent)
			}
		})
	}
}

func TestRunQuery_CostGate_RejectsMultiStatement(t *testing.T) {
	db := &costDB{limit: 1000, plan: pgPlanCheap}
	uc := newCostUseCase(db, "postgres")

	_, err := uc.ExecuteQueryMasked(context.Background(), "pg1", "SELECT 1; DROP TABLE t", nil, false)
	if err == nil || !strings.Contains(err.Error(), "exactly one statement") {
		t.Fatalf("expected single-statement denial, got: %v", err)
	}
	if len(db.sent) != 0 {
		t.Fatalf("no statement may be sent to the database, sent: %q", db.sent)
	}
}

func TestRunQuery_CostGate_InternalQueriesNotGated(t *testing.T) {
	db := &costDB{limit: 1000, plan: pgPlanHuge}
	uc := newCostUseCase(db, "postgres")

	if _, err := uc.ExecuteQuery(context.Background(), "pg1", "SELECT * FROM a CROSS JOIN b", nil); err != nil {
		t.Fatalf("internal ExecuteQuery must not be gated, got: %v", err)
	}
	if db.explainCount() != 0 {
		t.Fatalf("internal query must not trigger EXPLAIN, sent: %q", db.sent)
	}
}

func TestExecuteExplain_AnalyzeIsGated(t *testing.T) {
	const stmt = "SELECT * FROM a CROSS JOIN b"

	db := &costDB{limit: 1000, plan: pgPlanHuge}
	uc := newCostUseCase(db, "postgres")

	_, err := uc.ExecuteExplain(context.Background(), "pg1", stmt, true)
	if err == nil || !strings.Contains(err.Error(), "exceeds max_query_cost") {
		t.Fatalf("EXPLAIN ANALYZE above the limit must be denied, got: %v", err)
	}
	for _, q := range db.sent {
		if strings.Contains(q, "ANALYZE") {
			t.Fatalf("EXPLAIN ANALYZE must not reach the engine, sent: %q", db.sent)
		}
	}

	// Plain EXPLAIN does not execute the statement, so it stays available.
	if _, err := uc.ExecuteExplain(context.Background(), "pg1", stmt, false); err != nil {
		t.Fatalf("plain EXPLAIN must not be gated, got: %v", err)
	}
}
