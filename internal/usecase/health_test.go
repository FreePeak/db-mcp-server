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
