package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSharedBuffersProbe proves the probe reads both pool size and
// database size in one round trip, PostgreSQL only.
func TestSharedBuffersProbe(t *testing.T) {
	q := sharedBuffersQuery("postgres")
	if !strings.Contains(q, "shared_buffers") || !strings.Contains(q, "pg_database_size") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if sharedBuffersQuery("mysql") != "" || sharedBuffersQuery("sqlite") != "" {
		t.Fatal("only postgres exposes shared_buffers")
	}
}

// TestSharedBuffersVerdict proves the sizing ladder.
func TestSharedBuffersVerdict(t *testing.T) {
	if got := sharedBuffersVerdict(8<<30, 1<<30); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := sharedBuffersVerdict(128<<20, 20<<30)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "ALTER SYSTEM") {
		t.Fatalf("undersized pool not escalated:\n%s", got)
	}
	if !strings.Contains(got, "MB") && !strings.Contains(got, "GB") {
		t.Fatalf("warning lacks readable sizes:\n%s", got)
	}
}

// TestAuditSharedBuffers_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditSharedBuffers_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditSharedBuffers(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
