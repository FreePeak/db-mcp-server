package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestArchiverCatalog proves the SELECT reads pg_stat_archiver's
// success and failure counters.
func TestArchiverCatalog(t *testing.T) {
	q := archiverQuery("postgres")
	for _, want := range []string{"pg_stat_archiver", "failed_count", "last_failed_wal"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if archiverQuery("mysql") != "" || archiverQuery("sqlite") != "" {
		t.Fatal("only postgres has pg_stat_archiver")
	}
}

// TestCheckWALArchive_Unsupported proves unsupported engines get an
// explicit error.
func TestCheckWALArchive_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckWALArchive(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestArchiverVerdict proves failure escalation.
func TestArchiverVerdict(t *testing.T) {
	if got := archiverVerdict(100, 0, "", ""); !strings.Contains(got, "healthy") {
		t.Fatalf("no failures misjudged:\n%s", got)
	}
	if got := archiverVerdict(90, 7, "2026-08-22", "000000010000000000000042"); !strings.Contains(got, "FAILING") || !strings.Contains(got, "00000042") {
		t.Fatalf("failures not escalated:\n%s", got)
	}
	if got := archiverVerdict(0, 0, "", ""); !strings.Contains(got, "never archived") {
		t.Fatalf("idle state misjudged:\n%s", got)
	}
}
