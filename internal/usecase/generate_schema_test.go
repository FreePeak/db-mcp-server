package usecase

import (
	"context"
	"strings"
	"testing"
)

func generateSchemaTestDB(t *testing.T) *DatabaseUseCase {
	t.Helper()
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE order_items (id INTEGER PRIMARY KEY, sku TEXT, qty INTEGER, price REAL)`)
	return NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
}

// TestGenerateSchemaCode_Go proves cycle 59: live schema renders as Go
// structs with db tags and engine-type mapping.
func TestGenerateSchemaCode_Go(t *testing.T) {
	uc := generateSchemaTestDB(t)
	out, err := uc.GenerateSchemaCode(context.Background(), "db1", "go")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	for _, want := range []string{
		"type OrderItems struct {",
		"ID    int64 `db:\"id\"`",
		"Sku   string `db:\"sku\"`",
		"Qty   int64 `db:\"qty\"`",
		"Price float64 `db:\"price\"`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

// TestGenerateSchemaCode_TypeScript proves the TS target emits interfaces
// with number/string mappings.
func TestGenerateSchemaCode_TypeScript(t *testing.T) {
	uc := generateSchemaTestDB(t)
	out, err := uc.GenerateSchemaCode(context.Background(), "db1", "typescript")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	for _, want := range []string{
		"export interface OrderItems {",
		"id: number;",
		"sku: string;",
		"qty: number;",
		"price: number;",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestGenerateSchemaCode_Errors(t *testing.T) {
	uc := generateSchemaTestDB(t)
	if _, err := uc.GenerateSchemaCode(context.Background(), "db1", "cobol"); err == nil {
		t.Fatal("unknown target must error")
	}
}
