package usecase

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestEngineSlowQueries_SQLiteGracefulDegradation proves the engine_slow_queries
// action degrades to an actionable note on engines without statement catalogs.
func TestEngineSlowQueries_SQLiteGracefulDegradation(t *testing.T) {
	raw := openSQLiteForTest(t)
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	out, err := uc.AnalyzePerformance(context.Background(), "db1", "engine_slow_queries", "", 5, 0)
	if err != nil {
		t.Fatalf("engine_slow_queries should not error, got: %v", err)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "sqlite") && !strings.Contains(lower, "not available") && !strings.Contains(lower, "unavailable") {
		t.Fatalf("expected graceful degradation note mentioning the limitation:\n%s", out)
	}
}

// TestFirstNumericField extracts the first numeric line from engine output.
func TestFirstNumericField(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"numeric_first", "cache hit ratio\n95.2\nother", "95.2"},
		{"skip_text_lines", "\nbuffer stats:\n87\n", "87"},
		{"none_numeric", "no numbers here\n", ""},
		{"empty_input", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNumericField(tt.in); got != tt.want {
				t.Fatalf("firstNumericField(%q) = %q want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestEnableMaskingAuditFile_BadPathErrors proves invalid sink paths fail
// with a clear error instead of silently disabling durability.
func TestEnableMaskingAuditFile_BadPathErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "missing-dir", "audit.jsonl") // parent does not exist
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	if err := uc.EnableMaskingAuditFile(bad); err == nil {
		t.Fatal("expected error for unwritable sink path")
	}
}

// TestMaskingAuditFile_RoundTripAfterClose proves a closed sink can be
// re-enabled (operator rotation scenario).
func TestMaskingAuditFile_RoundTripAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit2.jsonl")
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	if err := uc.EnableMaskingAuditFile(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if err := uc.CloseMaskingAuditFile(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := uc.EnableMaskingAuditFile(path); err != nil {
		t.Fatalf("re-enable after close failed: %v", err)
	}
}

// TestSQLGuard_DollarQuotedStrings pins Postgres dollar-quoting so bodies of
// $$...$$ literals never count as SQL keywords or WHERE clauses.
func TestSQLGuard_DollarQuotedStrings(t *testing.T) {
	// A SELECT whose dollar-quoted body contains UPDATE-like text stays read-only.
	q := `SELECT * FROM t WHERE note = $tag$UPDATE users SET admin=true; DROP TABLE x$tag$`
	if IsWriteStatement(q) {
		t.Fatalf("dollar-quoted literal content must not classify as write: %q", q)
	}

	// A real write wrapped around dollar quotes still classifies as write.
	w := `UPDATE t SET note = $a$SELECT 1$a$ WHERE id = 1`
	if !IsWriteStatement(w) {
		t.Fatalf("real UPDATE misclassified as read: %q", w)
	}
}

// TestGetDatabaseTypePassthrough covers the trivial repo passthrough used by
// every engine-specific code path.
func TestGetDatabaseTypePassthrough(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: openSQLiteForTest(t)}, dbType: "sqlite"})
	got, err := uc.GetDatabaseType("db1")
	if err != nil || got != "sqlite" {
		t.Fatalf("GetDatabaseType = %q, %v", got, err)
	}
}
