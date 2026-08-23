package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestSeqScanCatalog proves per-engine sequential-scan SELECTs read the
// workload counters.
func TestSeqScanCatalog(t *testing.T) {
	pg := seqScanQuery("postgres")
	if !strings.Contains(pg, "pg_stat_user_tables") || !strings.Contains(pg, "seq_scan") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := seqScanQuery("mysql")
	if !strings.Contains(my, "table_io_waits_summary_by_index_usage") ||
		!strings.Contains(my, "INDEX_NAME IS NULL") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if seqScanQuery("sqlite") != "" {
		t.Fatal("sqlite should have no seq-scan catalog")
	}
}

// TestFindSeqScanHeavy_Unsupported proves unsupported engines get an
// explicit error.
func TestFindSeqScanHeavy_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.FindSeqScanHeavy(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestRenderSeqScanVerdict proves the seq-vs-index verdict logic.
func TestRenderSeqScanVerdict(t *testing.T) {
	if got := seqScanVerdict(500, 2); !strings.Contains(got, "indexing candidate") {
		t.Fatalf("seq-heavy table not flagged:\n%s", got)
	}
	if got := seqScanVerdict(10, 400); !strings.Contains(got, "index access dominates") {
		t.Fatalf("healthy table misflagged:\n%s", got)
	}
	if got := seqScanVerdict(0, 0); !strings.Contains(got, "no scans recorded") {
		t.Fatalf("cold table wrong:\n%s", got)
	}
}
