package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestExtensionCatalog proves the SELECT covers installed and
// available extensions.
func TestExtensionCatalog(t *testing.T) {
	q := extensionQuery("postgres")
	for _, want := range []string{"pg_extension", "pg_available_extensions"} {
		if !strings.Contains(q, want) {
			t.Fatalf("catalog missing %q:\n%s", want, q)
		}
	}
	if extensionQuery("mysql") != "" || extensionQuery("sqlite") != "" {
		t.Fatal("only postgres has a registry of extensions")
	}
}

// TestListExtensions_Unsupported proves unsupported engines get an
// explicit error.
func TestListExtensions_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListExtensions(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
