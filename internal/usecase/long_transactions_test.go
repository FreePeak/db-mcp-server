package usecase

import (
	"context"
	"strings"
	"testing"
)

func nil2ctx() context.Context { return context.Background() }

// TestLongTransactionsCatalog proves per-engine transaction-age SELECTs
// exist and target the right catalogs.
func TestLongTransactionsCatalog(t *testing.T) {
	pg := longTransactionsQuery("postgres")
	if !strings.Contains(pg, "idle in transaction") || !strings.Contains(pg, "xact_start") {
		t.Fatalf("pg catalog wrong:\n%s", pg)
	}
	my := longTransactionsQuery("mysql")
	if !strings.Contains(my, "innodb_trx") || !strings.Contains(my, "trx_started") {
		t.Fatalf("mysql catalog wrong:\n%s", my)
	}
	if longTransactionsQuery("sqlite") != "" {
		t.Fatal("sqlite should have no long-transaction catalog")
	}
}

// TestListLongTransactions_Unsupported proves unsupported engines get an
// explicit error rather than fabricated output.
func TestListLongTransactions_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ListLongTransactions(nil2ctx(), "db1", 60); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}
