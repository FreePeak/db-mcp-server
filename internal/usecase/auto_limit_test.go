package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestApplyAutoLimit(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{
			"plain_select",
			"SELECT * FROM users", 10,
			"SELECT * FROM users LIMIT 10",
		},
		{
			"existing_top_limit_untouched",
			"SELECT * FROM users LIMIT 5", 10,
			"SELECT * FROM users LIMIT 5",
		},
		{
			"subquery_limit_still_injected",
			"SELECT * FROM (SELECT * FROM t LIMIT 3) x", 10,
			"SELECT * FROM (SELECT * FROM t LIMIT 3) x LIMIT 10",
		},
		{
			"order_by_appends",
			"SELECT id FROM events ORDER BY created_at DESC", 25,
			"SELECT id FROM events ORDER BY created_at DESC LIMIT 25",
		},
		{
			"trailing_semicolon",
			"SELECT * FROM t;", 5,
			"SELECT * FROM t LIMIT 5",
		},
		{
			"where_clause",
			"SELECT * FROM t WHERE a = 1 AND b = 2", 7,
			"SELECT * FROM t WHERE a = 1 AND b = 2 LIMIT 7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyAutoLimit(tt.in, tt.limit)
			if got != tt.want {
				t.Fatalf("applyAutoLimit(%q, %d) =\n %q\nwant\n%q", tt.in, tt.limit, got, tt.want)
			}
		})
	}

	t.Run("non_select_untouched", func(t *testing.T) {
		for _, q := range []string{"UPDATE t SET a = 1", "INSERT INTO t VALUES (1)", "DELETE FROM t", "CREATE TABLE x (a INT)"} {
			if got := applyAutoLimit(q, 10); got != q {
				t.Fatalf("non-SELECT modified: %q -> %q", q, got)
			}
		}
	})

	t.Run("zero_limit_noop", func(t *testing.T) {
		q := "SELECT * FROM users"
		if got := applyAutoLimit(q, 0); got != q {
			t.Fatalf("limit=0 must be a no-op: %q", got)
		}
	})
}

// TestHasTopLevelLimit distinguishes outer from subquery limits.
func TestHasTopLevelLimit(t *testing.T) {
	if !hasTopLevelLimit("SELECT * FROM t LIMIT 5") {
		t.Fatal("top-level LIMIT not detected")
	}
	if hasTopLevelLimit("SELECT * FROM (SELECT * FROM u LIMIT 3) x") {
		t.Fatal("subquery LIMIT misdetected as top-level")
	}
	if hasTopLevelLimit("SELECT 'LIMIT' AS word FROM t") {
		t.Fatal("LIMIT inside string literal misdetected")
	}
}

// TestAutoLimitedQuery_EndToEnd proves a SELECT against a max_rows'd database
// still returns correct rows with injection active.
func TestAutoLimitedQuery_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE nums (n INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 1; i <= 50; i++ {
		if _, err := raw.Exec(`INSERT INTO nums (n) VALUES (?)`, i); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	wrapper := &maxRowsSQLite{sqliteDB{db: raw}, 10}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.ExecuteQuery(context.Background(), "db1", "SELECT n FROM nums ORDER BY n", nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	// Injection succeeded if exactly 10 rows arrive (no client-side truncation
	// needed) and row 11 is absent.
	if !strings.Contains(out, "Total rows: 10") || strings.Contains(out, "Truncated") {
		t.Fatalf("expected exactly 10 rows via injected LIMIT:\n%s", out)
	}
	if strings.Contains(out, "\n11\n") {
		t.Fatalf("row 11 should not be present:\n%s", out)
	}
}

type maxRowsSQLite struct {
	sqliteDB
	mr int
}

func (m *maxRowsSQLite) MaxRows() int { return m.mr }

// TestApplyOracleRowLimit covers ROWNUM wrapping for engines without LIMIT.
// The wrap must preserve WITH clauses, skip non-SELECT statements and queries
// that already carry a top-level bound (ROWNUM or FETCH FIRST).
func TestApplyOracleRowLimit(t *testing.T) {
	wrap := func(inner string) string {
		return fmt.Sprintf("SELECT * FROM (%s) WHERE ROWNUM <= %d", inner, 10)
	}
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{
			"plain_select_wrapped",
			"SELECT * FROM users", 10,
			wrap("SELECT * FROM users"),
		},
		{
			"with_clause_wrapped",
			"WITH recent AS (SELECT * FROM events) SELECT * FROM recent", 10,
			wrap("WITH recent AS (SELECT * FROM events) SELECT * FROM recent"),
		},
		{
			"order_by_preserved_inside_wrap",
			"SELECT id FROM logs ORDER BY created_at DESC", 10,
			wrap("SELECT id FROM logs ORDER BY created_at DESC"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyOracleRowLimit(tt.in, tt.limit)
			if got != tt.want {
				t.Fatalf("applyOracleRowLimit(%q, %d) =\n%q\nwant\n%q", tt.in, tt.limit, got, tt.want)
			}
		})
	}

	t.Run("non_select_untouched", func(t *testing.T) {
		for _, q := range []string{"UPDATE t SET a = 1", "DELETE FROM t", "BEGIN foo; END;"} {
			if got := applyOracleRowLimit(q, 10); got != q {
				t.Fatalf("non-SELECT modified: %q -> %q", q, got)
			}
		}
	})

	t.Run("zero_limit_noop", func(t *testing.T) {
		q := "SELECT * FROM t"
		if got := applyOracleRowLimit(q, 0); got != q {
			t.Fatalf("limit=0 must be a no-op: %q", got)
		}
	})

	t.Run("already_bounded_untouched", func(t *testing.T) {
		for _, q := range []string{
			"SELECT * FROM (SELECT * FROM t) WHERE ROWNUM <= 5",
			"SELECT * FROM orders FETCH FIRST 20 ROWS ONLY",
		} {
			if got := applyOracleRowLimit(q, 10); got != q {
				t.Fatalf("bounded query modified: %q -> %q", q, got)
			}
		}
	})

	t.Run("subquery_rownum_still_wrapped", func(t *testing.T) {
		in := "SELECT * FROM (SELECT * FROM u WHERE ROWNUM <= 3)"
		want := "SELECT * FROM (" + in + ") WHERE ROWNUM <= 10"
		if got := applyOracleRowLimit(in, 10); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("literal_rownum_still_wrapped", func(t *testing.T) {
		in := "SELECT 'ROWNUM' AS word FROM dual"
		want := "SELECT * FROM (" + in + ") WHERE ROWNUM <= 10"
		if got := applyOracleRowLimit(in, 10); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestAutoLimitedQuery_OracleRoutesToRownumWrap locks in cycle 54: the Oracle
// branch of autoLimitedQuery must ROWNUM-wrap instead of silently skipping
// injection (the pre-cycle-54 behavior left unbounded SELECTs unbounded).
func TestAutoLimitedQuery_OracleRoutesToRownumWrap(t *testing.T) {
	db := &maxRowsSQLite{mr: 10}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "oracle"})

	got := uc.autoLimitedQuery("ora1", "SELECT name FROM accounts", db)
	want := "SELECT * FROM (SELECT name FROM accounts) WHERE ROWNUM <= 10"
	if got != want {
		t.Fatalf("oracle query not wrapped:\n got %q\nwant %q", got, want)
	}

	// LIMIT dialects keep the appended-LIMIT path.
	ucPG := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})
	got = ucPG.autoLimitedQuery("pg1", "SELECT name FROM accounts", db)
	if want := "SELECT name FROM accounts LIMIT 10"; got != want {
		t.Fatalf("postgres query wrong:\n got %q\nwant %q", got, want)
	}
}
