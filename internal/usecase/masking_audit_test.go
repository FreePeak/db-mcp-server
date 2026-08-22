package usecase

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestMaskingAudit_RecordsRedactions proves masked queries append audit
// events retrievable per database.
func TestMaskingAudit_RecordsRedactions(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE acct (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO acct (email) VALUES ('a@x.io'), ('b@y.io')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	before := time.Now().Add(-time.Second)
	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM acct", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	events := uc.GetMaskingAudit("db1")
	if len(events) != 1 {
		t.Fatalf("expected exactly 1 audit event, got %d", len(events))
	}
	ev := events[0]
	if ev.DatabaseID != "db1" || ev.CellsMasked == 0 {
		t.Fatalf("unexpected event: %+v", ev)
	}
	if ev.Timestamp.Before(before) {
		t.Fatalf("event timestamp predates query: %v", ev.Timestamp)
	}
	if !strings.Contains(ev.Query, "SELECT email") {
		t.Fatalf("event should carry the query text: %+v", ev)
	}
}

// TestMaskingAudit_UnmaskedQueryLeavesNoEvent proves the audit log tracks
// actual redactions, not all queries.
func TestMaskingAudit_UnmaskedQueryLeavesNoEvent(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT 1", nil, false); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if events := uc.GetMaskingAudit("db1"); len(events) != 0 {
		t.Fatalf("unmasked query must not create audit events, got %d", len(events))
	}
}

// TestMaskingAudit_RingBufferCap proves the log is bounded and keeps the
// most recent events.
func TestMaskingAudit_RingBufferCap(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t (email) VALUES ('x@y.z')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	for i := 0; i < maskingAuditCapacity+10; i++ {
		if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM t", nil, true); err != nil {
			t.Fatalf("query %d failed: %v", i, err)
		}
	}
	events := uc.GetMaskingAudit("db1")
	if len(events) != maskingAuditCapacity {
		t.Fatalf("expected ring buffer capped at %d, got %d", maskingAuditCapacity, len(events))
	}
}

// TestMaskingAudit_IsolatedPerDatabase proves events don't leak across DBs.
func TestMaskingAudit_IsolatedPerDatabase(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if _, err := uc.ExecuteQueryMasked(context.Background(), "alpha", "SELECT email FROM t", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if events := uc.GetMaskingAudit("beta"); len(events) != 0 {
		t.Fatalf("events leaked across databases: %+v", events)
	}
}

// TestMaskingAudit_SurfaceInHealth proves the health payload exposes recent
// redaction events for operator visibility.
func TestMaskingAudit_SurfaceInHealth(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t (email) VALUES ('a@b.c')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM t", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	info, err := uc.HealthCheck(context.Background(), "db1")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if n, ok := info["masking_events_recent"].(int); !ok || n != 1 {
		t.Fatalf("expected masking_events_recent=1, got %v", info["masking_events_recent"])
	}
	if _, ok := info["masking_events_last"].(MaskingAuditEvent); !ok {
		t.Fatalf("expected masking_events_last event, got %T", info["masking_events_last"])
	}
}
