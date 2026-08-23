package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestCheckpointTemplates proves both PG generations are covered
// (PG17+ moved checkpoint stats to pg_stat_checkpointer).
func TestCheckpointTemplates(t *testing.T) {
	qs := checkpointQueries("postgres")
	if len(qs) != 2 {
		t.Fatalf("want modern+legacy templates, got %d:\n%v", len(qs), qs)
	}
	if !strings.Contains(qs[0], "pg_stat_checkpointer") || !strings.Contains(qs[0], "checkpoints_timed") {
		t.Fatalf("modern template wrong:\n%s", qs[0])
	}
	if !strings.Contains(qs[1], "pg_stat_bgwriter") || !strings.Contains(qs[1], "checkpoints_timed") {
		t.Fatalf("legacy fallback wrong:\n%s", qs[1])
	}
	if len(checkpointQueries("sqlite")) != 0 || len(checkpointQueries("mysql")) != 0 {
		t.Fatal("only postgres exposes checkpoint counters")
	}
}

// TestCheckpointVerdict proves the req-ratio escalation.
func TestCheckpointVerdict(t *testing.T) {
	if got := checkpointVerdict(1000, 10); !strings.Contains(got, "healthy") {
		t.Fatalf("low req ratio misjudged:\n%s", got)
	}
	if got := checkpointVerdict(100, 50); !strings.Contains(got, "PRESSURE") || !strings.Contains(got, "max_wal_size") {
		t.Fatalf("high req ratio not escalated:\n%s", got)
	}
	if got := checkpointVerdict(0, 0); got == "" {
		t.Fatal("empty verdict")
	}
}

// TestCheckCheckpointPressure_Unsupported proves unsupported engines
// get an explicit error.
func TestCheckCheckpointPressure_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckCheckpointPressure(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
