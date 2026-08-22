package usecase

import (
	"context"
	"strings"
	"testing"
)

// fixtureRows builds a staticRows with one huge TEXT cell plus normal data.
func wideFixture() *staticRows {
	big := strings.Repeat("x", 2000)
	return &staticRows{
		columns: []string{"id", "title", "body"},
		data: [][]interface{}{
			{int64(1), "short title", big},
			{int64(2), "another", "tiny"},
		},
	}
}

// TestRenderQueryResults_VerbosityNormalTruncatesCells proves normal mode
// caps cell size while preserving every row and the column structure.
func TestRenderQueryResults_VerbosityNormalTruncatesCells(t *testing.T) {
	out, _, err := renderQueryResults(wideFixture(), 0, false, VerbosityNormal)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(out, "…(") || !strings.Contains(out, "chars)") {
		t.Fatalf("expected truncation marker like …(+1950 chars):\n%.400s", out)
	}
	if strings.Count(out, "\n") < 4 { // header + separator + 2 rows + totals
		t.Fatalf("rows must survive truncation:\n%s", out)
	}
	if !strings.Contains(out, "short title") || !strings.Contains(out, "tiny") {
		t.Fatalf("short cells must pass through untouched:\n%s", out)
	}
}

// TestRenderQueryResults_VerbosityMinimal proves minimal mode collapses the
// payload to row count plus a single-row preview.
func TestRenderQueryResults_VerbosityMinimal(t *testing.T) {
	out, _, err := renderQueryResults(wideFixture(), 0, false, VerbosityMinimal)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(out, "Total rows: 2") {
		t.Fatalf("minimal must report total rows:\n%s", out)
	}
	if strings.Contains(out, "another") {
		t.Fatalf("minimal must not include second row content:\n%s", out)
	}
	if !strings.Contains(out, "first row:") {
		t.Fatalf("minimal must include a first-row preview:\n%s", out)
	}
	if len(out) > 800 {
		t.Fatalf("minimal output should be compact, got %d bytes", len(out))
	}
}

// TestRenderQueryResults_VerbosityFullUnchanged proves full mode preserves
// byte-for-byte behavior of the legacy path (backward compat guarantee).
func TestRenderQueryResults_VerbosityFullUnchanged(t *testing.T) {
	fullOut, _, err := renderQueryResults(wideFixture(), 0, false, VerbosityFull)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	legacyOut, err := formatQueryResults(wideFixture(), 0)
	if err != nil {
		t.Fatalf("legacy render failed: %v", err)
	}
	if fullOut != legacyOut {
		t.Fatalf("full mode diverged from legacy renderer")
	}
	if !strings.Contains(fullOut, strings.Repeat("x", 2000)) {
		t.Fatal("full mode truncated a cell it must not truncate")
	}
}

// TestRenderQueryResults_MaxRowsAndVerbosityCombine proves guardrails stack.
func TestRenderQueryResults_MaxRowsAndVerbosityCombine(t *testing.T) {
	rows := &staticRows{
		columns: []string{"id"},
		data: [][]interface{}{
			{int64(1)}, {int64(2)}, {int64(3)},
		},
	}
	out, _, err := renderQueryResults(rows, 2, false, VerbosityMinimal)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	// Minimal counts every row and reports the cap honestly.
	if !strings.Contains(out, "max_rows=2 exceeded") {
		t.Fatalf("max_rows overflow notice expected:\n%s", out)
	}
}

// TestExecuteQueryVerbosity_EndToEnd runs the full path on real SQLite.
func TestExecuteQueryVerbosity_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE posts (id INTEGER PRIMARY KEY, body TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO posts (body) VALUES (?)`, strings.Repeat("z", 3000)); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	minimal, err := uc.ExecuteQueryVerbosity(context.Background(), "db1", "SELECT * FROM posts", nil, VerbosityMinimal)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(minimal) > 600 {
		t.Fatalf("minimal end-to-end output too large: %d bytes", len(minimal))
	}
	if !strings.Contains(minimal, "Total rows: 1") {
		t.Fatalf("expected compact summary:\n%s", minimal)
	}
}
