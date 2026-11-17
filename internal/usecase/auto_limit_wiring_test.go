package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// recordingDB wraps fakeDB and records the SQL text and arguments it was asked
// to run. When err is set, Query fails with it instead of returning rows.
type recordingDB struct {
	*fakeDB
	sql  string
	args []interface{}
	err  error
}

func (r *recordingDB) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	r.sql = query
	r.args = args
	if r.err != nil {
		return nil, r.err
	}
	return r.fakeDB.Query(ctx, query, args...)
}

// cappedSQLite is a real SQLite database with a configurable max_rows.
type cappedSQLite struct {
	*sqliteDB
	maxRows int
}

func (c cappedSQLite) MaxRows() int { return c.maxRows }

func TestAutoLimit_ExecuteQueryMaskedAppliesRowCap(t *testing.T) {
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 10}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})

	params := []interface{}{"x", 1}
	if _, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM t WHERE a = ? AND b = ?", params, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "SELECT * FROM t WHERE a = ? AND b = ?\nLIMIT 11"; db.sql != want {
		t.Fatalf("executed SQL\n got: %q\nwant: %q", db.sql, want)
	}
	if !reflect.DeepEqual(db.args, params) {
		t.Fatalf("executed args\n got: %v\nwant: %v", db.args, params)
	}
}

func TestAutoLimit_FailedCappedQueryNamesAppendedLimit(t *testing.T) {
	dbErr := errors.New("boom")
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 10}, err: dbErr}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})

	_, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM t", nil, false)
	if !errors.Is(err, dbErr) {
		t.Fatalf("error does not wrap the database error: %v", err)
	}
	for _, note := range []string{"LIMIT 11", "add an explicit LIMIT to send the query unchanged"} {
		if !strings.Contains(err.Error(), note) {
			t.Fatalf("error missing %q: %v", note, err)
		}
	}
}

func TestAutoLimit_FailedUnchangedQueryOmitsNote(t *testing.T) {
	dbErr := errors.New("boom")
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 10}, err: dbErr}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})

	_, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM t LIMIT 5", nil, false)
	if !errors.Is(err, dbErr) {
		t.Fatalf("error does not wrap the database error: %v", err)
	}
	if strings.Contains(err.Error(), "max_rows appended") {
		t.Fatalf("error should not mention an appended LIMIT: %v", err)
	}
}

func TestAutoLimit_ExecuteQueryInternalPathNeverRewrites(t *testing.T) {
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 10}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})

	if _, err := uc.ExecuteQuery(context.Background(), "db", "SELECT * FROM t", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "SELECT * FROM t"; db.sql != want {
		t.Fatalf("executed SQL\n got: %q\nwant: %q", db.sql, want)
	}
}

func TestAutoLimit_ZeroMaxRowsNeverRewrites(t *testing.T) {
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 0}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})

	if _, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM t", nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "SELECT * FROM t"; db.sql != want {
		t.Fatalf("executed SQL\n got: %q\nwant: %q", db.sql, want)
	}
}

func TestAutoLimit_OracleIsNeverRewritten(t *testing.T) {
	db := &recordingDB{fakeDB: &fakeDB{maxRows: 10}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "oracle"})

	if _, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT * FROM t", nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "SELECT * FROM t"; db.sql != want {
		t.Fatalf("executed SQL\n got: %q\nwant: %q", db.sql, want)
	}
}

func TestAutoLimit_SQLiteTruncatesAtMaxRows(t *testing.T) {
	// The recursive CTE never ends on its own. This is an end-to-end check that
	// the agent path returns promptly with the truncation notice. It does not
	// prove the LIMIT was sent: formatQueryResults stops reading at max_rows
	// and SQLite is pull-based, so the uncapped run gives the same output.
	// TestAutoLimit_ExecuteQueryMaskedAppliesRowCap asserts the executed SQL.
	db := cappedSQLite{sqliteDB: &sqliteDB{db: openSQLiteForTest(t)}, maxRows: 10}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "sqlite"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := uc.ExecuteQueryMasked(ctx, "db", "WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c) SELECT x FROM c", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := dataRowCount(out); got != 10 {
		t.Fatalf("data rows: got %d, want 10\n%s", got, out)
	}
	for _, notice := range []string{
		"Truncated: showing first 10 rows (max_rows=10)",
		"Total rows shown: 10",
	} {
		if !strings.Contains(out, notice) {
			t.Fatalf("output missing %q\n%s", notice, out)
		}
	}
}

func TestAutoLimit_SQLiteExactlyMaxRowsIsNotTruncated(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec("CREATE TABLE t (n INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := raw.Exec("WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 10) INSERT INTO t (n) SELECT x FROM c"); err != nil {
		t.Fatalf("seed rows: %v", err)
	}
	db := cappedSQLite{sqliteDB: &sqliteDB{db: raw}, maxRows: 10}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "sqlite"})

	out, err := uc.ExecuteQueryMasked(context.Background(), "db", "SELECT n FROM t ORDER BY n", nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := dataRowCount(out); got != 10 {
		t.Fatalf("data rows: got %d, want 10\n%s", got, out)
	}
	if strings.Contains(out, "Truncated:") {
		t.Fatalf("exactly max_rows rows must not be reported as truncated\n%s", out)
	}
}

// dataRowCount counts the rows printed under the result header, i.e. the
// non-empty lines between the dash separator and the first blank line.
func dataRowCount(out string) int {
	lines := strings.Split(out, "\n")
	count, inRows := 0, false
	for _, line := range lines {
		if strings.HasPrefix(line, "----") {
			inRows = true
			continue
		}
		if !inRows {
			continue
		}
		if line == "" {
			break
		}
		count++
	}
	return count
}
