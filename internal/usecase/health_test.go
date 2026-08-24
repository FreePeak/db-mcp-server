package usecase

import (
	"context"
	"testing"
)

// TestHealthCheck_EndToEnd runs a real health check against in-memory
// SQLite through the full use-case path.
func TestHealthCheck_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	_ = raw // ping and pool stats flow through sqliteDB's underlying handle

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	info, err := uc.HealthCheck(context.Background(), "sqlite1")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if healthy, ok := info["healthy"].(bool); !ok || !healthy {
		t.Fatalf("expected healthy=true, got %v", info["healthy"])
	}
	if _, ok := info["checked_at"]; !ok {
		t.Fatal("expected checked_at timestamp")
	}
}

// TestHealthCheck_GuardrailsVisible locks in cycle 21: health output must
// surface the active guardrails (read-only flag, row cap, statement timeout)
// so operators can verify enforcement instead of trusting config files.
func TestHealthCheck_GuardrailsVisible(t *testing.T) {
	db := &guardedDB{fakeDB: &fakeDB{readOnly: true, maxRows: 100}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	info, err := uc.HealthCheck(context.Background(), "guarded_db")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if ro, ok := info["read_only"].(bool); !ok || !ro {
		t.Errorf("expected read_only=true, got %v", info["read_only"])
	}
	if mr, ok := info["max_rows"].(int); !ok || mr != 100 {
		t.Errorf("expected max_rows=100, got %v", info["max_rows"])
	}
	if to, ok := info["statement_timeout_seconds"].(int); !ok || to != 30 {
		t.Errorf("expected statement_timeout_seconds=30, got %v", info["statement_timeout_seconds"])
	}
}

// guardedDB adds guardrail capabilities on top of fakeDB.
type guardedDB struct {
	*fakeDB
}

func (g *guardedDB) QueryTimeout() int { return 30 }

// TestHealthCheck_UnreachableDatabaseReportsUnhealthy verifies a failing
// probe marks the database unhealthy instead of erroring out.
func TestHealthCheck_UnreachableDatabaseReportsUnhealthy(t *testing.T) {
	db := &unpingableDB{fakeDB: &fakeDB{}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	info, err := uc.HealthCheck(context.Background(), "dead_db")
	if err != nil {
		t.Fatalf("health check should not error on unhealthy database: %v", err)
	}
	if healthy, ok := info["healthy"].(bool); !ok || healthy {
		t.Fatalf("expected healthy=false, got %v", info["healthy"])
	}
	if errMsg, ok := info["error"].(string); !ok || errMsg == "" {
		t.Fatalf("expected probe error detail, got %v", info["error"])
	}
}

// unpingableDB wraps a fakeDB whose Ping always fails.
type unpingableDB struct{ *fakeDB }

func (u *unpingableDB) Ping(ctx context.Context) error {
	return context.Canceled
}
