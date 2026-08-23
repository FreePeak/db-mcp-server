package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFlushNeighborsProbe proves the probe targets MySQL only.
func TestFlushNeighborsProbe(t *testing.T) {
	if q := flushNeighborsQuery("mysql"); !strings.Contains(q, "innodb_flush_neighbors") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if flushNeighborsQuery("postgres") != "" || flushNeighborsQuery("sqlite") != "" {
		t.Fatal("only mysql/mariadb expose innodb_flush_neighbors")
	}
}

// TestFlushNeighborsVerdict proves the escalation ladder.
func TestFlushNeighborsVerdict(t *testing.T) {
	if got := flushNeighborsVerdict(0); got != "" {
		t.Fatalf("SSD-tuned must render empty (audit adds the clean line), got:\n%s", got)
	}
	got := flushNeighborsVerdict(1)
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "spinning-disk") {
		t.Fatalf("default not escalated:\n%s", got)
	}
	if !strings.Contains(got, "SET GLOBAL innodb_flush_neighbors=0") {
		t.Fatalf("warning must name the live fix, got:\n%s", got)
	}
}

// TestAuditFlushNeighbors_Unsupported proves non-MySQL engines get an
// explicit error.
func TestAuditFlushNeighbors_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditFlushNeighbors(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
