package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestWalCompressionProbe proves the probe targets PostgreSQL only.
func TestWalCompressionProbe(t *testing.T) {
	if q := walCompressionQuery("postgres"); !strings.Contains(q, "wal_compression") {
		t.Fatalf("probe wrong:\n%s", q)
	}
	if walCompressionQuery("mysql") != "" || walCompressionQuery("sqlite") != "" {
		t.Fatal("only postgres exposes wal_compression")
	}
}

// TestWalCompressionVerdict proves the escalation ladder.
func TestWalCompressionVerdict(t *testing.T) {
	for _, ok := range []string{"on", "lz4", "zstd", "pglz"} {
		if got := walCompressionVerdict(ok); got != "" {
			t.Fatalf("healthy %q must render empty, got:\n%s", ok, got)
		}
	}
	got := walCompressionVerdict("off")
	if !strings.Contains(got, "WARNING") || !strings.Contains(got, "full-page") {
		t.Fatalf("off not escalated:\n%s", got)
	}
	if !strings.Contains(got, "ALTER SYSTEM") && !strings.Contains(got, "reload") {
		t.Fatalf("warning must name the fix path, got:\n%s", got)
	}
	if got := walCompressionVerdict(""); !strings.Contains(got, "unreadable") {
		t.Fatalf("empty misjudged:\n%s", got)
	}
}

// TestAuditWalCompression_Unsupported proves non-PG engines get an
// explicit error.
func TestAuditWalCompression_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditWalCompression(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
