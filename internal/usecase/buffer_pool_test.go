package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBufferPoolProbe proves the probe targets MySQL only and gathers
// both pool size and data volume in one round trip.
func TestBufferPoolProbe(t *testing.T) {
	q := bufferPoolQuery("mysql")
	if !strings.Contains(q, "innodb_buffer_pool_size") || !strings.Contains(q, "information_schema") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if bufferPoolQuery("postgres") != "" || bufferPoolQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_buffer_pool_size")
	}
}

// TestBufferPoolVerdict proves the sizing ladder.
func TestBufferPoolVerdict(t *testing.T) {
	if got := bufferPoolVerdict(8<<30, 1<<30); got != "" {
		t.Fatalf("healthy must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := bufferPoolVerdict(128<<20, 20<<30)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "SET GLOBAL") {
		t.Fatalf("undersized pool not escalated:\n%s", got)
	}
	if !strings.Contains(got, "humanBytes") && !strings.Contains(got, "MB") && !strings.Contains(got, "GB") {
		t.Fatalf("warning lacks readable sizes:\n%s", got)
	}
}

// TestAuditBufferPool_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditBufferPool_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditBufferPool(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
