package usecase

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestQueryHistory_RecordsBothPaths proves queries AND statements land in
// the history ring with duration and outcome.
func TestQueryHistory_RecordsBothPaths(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	before := time.Now().Add(-time.Second)
	if _, err := uc.ExecuteQuery(context.Background(), "db1", "SELECT id FROM t", nil); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if _, err := uc.ExecuteStatement(context.Background(), "db1", "INSERT INTO t VALUES (1)", nil); err != nil {
		t.Fatalf("statement failed: %v", err)
	}

	hist := uc.GetQueryHistory("db1")
	if len(hist) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(hist), hist)
	}
	if hist[0].Kind != "read" || !strings.Contains(hist[0].Statement, "SELECT") {
		t.Fatalf("first entry wrong: %+v", hist[0])
	}
	if hist[1].Kind != "write" || !strings.Contains(hist[1].Statement, "INSERT") {
		t.Fatalf("second entry wrong: %+v", hist[1])
	}
	for _, h := range hist {
		if h.Timestamp.Before(before) {
			t.Fatal("timestamp predates execution")
		}
		if h.DurationMs < 0 {
			t.Fatalf("negative duration: %+v", h)
		}
	}
}

// TestQueryHistory_CapturesFailures proves errors are recorded with ok=false.
func TestQueryHistory_CapturesFailures(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	if _, err := uc.ExecuteQuery(context.Background(), "db1", "SELECT * FROM missing_table", nil); err == nil {
		t.Fatal("expected error from bad query")
	}
	hist := uc.GetQueryHistory("db1")
	if len(hist) != 1 || hist[0].Success {
		t.Fatalf("failed query must be recorded with success=false: %+v", hist)
	}
}

// TestQueryHistory_RingCapAndIsolation proves bounded retention and per-db scoping.
func TestQueryHistory_RingCapAndIsolation(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	for i := 0; i < queryHistoryCapacity+15; i++ {
		_, _ = uc.ExecuteQuery(context.Background(), "db1", "SELECT 1", nil)
	}
	hist := uc.GetQueryHistory("db1")
	if len(hist) != queryHistoryCapacity {
		t.Fatalf("cap violated: %d > %d", len(hist), queryHistoryCapacity)
	}
	if other := uc.GetQueryHistory("other"); len(other) != 0 {
		t.Fatal("history leaked across databases")
	}
}
