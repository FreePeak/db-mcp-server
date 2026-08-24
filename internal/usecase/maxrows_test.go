package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeRows is an in-memory domain.Rows implementation for testing result
// formatting and the max_rows guardrail.
type fakeRows struct {
	columns []string
	data    [][]interface{}
	idx     int
	closed  bool
	scanErr error
	errErr  error
}

func (r *fakeRows) Close() error               { r.closed = true; return nil }
func (r *fakeRows) Columns() ([]string, error) { return r.columns, nil }
func (r *fakeRows) Next() bool                 { return r.idx < len(r.data) }
func (r *fakeRows) Scan(dest ...interface{}) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for i, d := range dest {
		p, ok := d.(*interface{})
		if !ok {
			return fmt.Errorf("fakeRows: dest[%d] is not *interface{}", i)
		}
		*p = r.data[r.idx][i]
	}
	r.idx++
	return nil
}
func (r *fakeRows) Err() error { return r.errErr }

func newFakeRows(n int) *fakeRows {
	data := make([][]interface{}, n)
	for i := range data {
		data[i] = []interface{}{fmt.Sprintf("val%d", i), i * 10}
	}
	return &fakeRows{
		columns: []string{"name", "num"},
		data:    data,
	}
}

// TestFormatQueryResults_NoLimit verifies unlimited output when maxRows <= 0.
func TestFormatQueryResults_NoLimit(t *testing.T) {
	rows := newFakeRows(5)
	out, err := formatQueryResults(rows, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Total rows: 5") {
		t.Fatalf("expected total row count in output, got:\n%s", out)
	}
	if strings.Contains(out, "Truncated") || strings.Contains(out, "truncated") {
		t.Fatalf("did not expect truncation notice without a limit, got:\n%s", out)
	}
	if strings.Count(out, "val") != 5 {
		t.Fatalf("expected all 5 rows in output, got:\n%s", out)
	}
}

// TestFormatQueryResults_TruncatesAtMaxRows locks in the max_rows guardrail:
// a query hitting a billion-row table must not flood the agent context window.
func TestFormatQueryResults_TruncatesAtMaxRows(t *testing.T) {
	rows := newFakeRows(100)
	out, err := formatQueryResults(rows, 3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(out, "val") != 3 {
		t.Fatalf("expected exactly 3 data rows in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Truncated") {
		t.Fatalf("expected truncation notice, got:\n%s", out)
	}
	if !strings.Contains(out, "max_rows=3") {
		t.Fatalf("expected notice to mention max_rows=3, got:\n%s", out)
	}
	if strings.Contains(out, "Total rows: ") && !strings.Contains(out, "Total rows shown: 3") {
		t.Fatalf("truncated output must report shown count, got:\n%s", out)
	}
}

// TestFormatQueryResults_LimitAboveRowCount ensures no false truncation.
func TestFormatQueryResults_LimitAboveRowCount(t *testing.T) {
	rows := newFakeRows(4)
	out, err := formatQueryResults(rows, 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(out, "val") != 4 {
		t.Fatalf("expected all 4 rows, got:\n%s", out)
	}
	if strings.Contains(out, "Truncated") {
		t.Fatalf("limit above row count must not truncate, got:\n%s", out)
	}
}

// TestFormatQueryResults_ScanErrorPropagates ensures scan failures surface.
func TestFormatQueryResults_ScanErrorPropagates(t *testing.T) {
	rows := &fakeRows{
		columns: []string{"a"},
		data:    [][]interface{}{{"x"}},
		scanErr: fmt.Errorf("boom"),
	}
	if _, err := formatQueryResults(rows, 0, nil); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected scan error to propagate, got %v", err)
	}
}

// TestExecuteQuery_MaxRowsFromDatabase verifies that ExecuteQuery pulls the
// row limit from the underlying database configuration end-to-end.
func TestExecuteQuery_MaxRowsFromDatabase(t *testing.T) {
	db := &fakeDB{
		maxRows:     2,
		queryResult: newFakeRows(50),
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	out, err := uc.ExecuteQuery(context.Background(), "pg_prod", "SELECT name FROM t", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.queryCalls != 1 {
		t.Fatalf("expected exactly one Query call, got %d", db.queryCalls)
	}
	if strings.Count(out, "val") != 2 {
		t.Fatalf("expected only max_rows=2 data rows, got:\n%s", out)
	}
	if !strings.Contains(out, "Truncated") {
		t.Fatalf("expected truncation notice, got:\n%s", out)
	}
}
