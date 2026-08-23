package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestCharsetCatalog proves the SELECT targets deprecated utf8mb3
// columns in the current schema.
func TestCharsetCatalog(t *testing.T) {
	q := charsetQuery("mysql")
	for _, want := range []string{"information_schema.COLUMNS", "utf8mb3", "DATABASE()"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if charsetQuery("sqlite") != "" || charsetQuery("postgres") != "" {
		t.Fatal("only mysql exposes per-column charsets")
	}
}

// TestAuditCharsets_Unsupported proves unsupported engines get an
// explicit error.
func TestAuditCharsets_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.AuditCharsets(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
