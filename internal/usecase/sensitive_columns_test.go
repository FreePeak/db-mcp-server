package usecase

import (
	"context"
	"strings"
	"testing"
)

// seedSensitiveSchema builds a table set with known PII carriers and benign
// columns to prove classification precision.
func seedSensitiveSchema(t *testing.T) (*DatabaseUseCase, context.Context) {
	t.Helper()
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	must(`CREATE TABLE customers (
		id INTEGER PRIMARY KEY,
		email TEXT,
		phone_number TEXT,
		ssn TEXT,
		card_token TEXT,
		full_name TEXT,
		birth_date TEXT,
		street_addr TEXT,
		iban_code TEXT
	)`)
	must(`CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		customer_id INTEGER,
		total REAL,
		status TEXT
	)`)

	wrapper := &sqliteDB{db: raw}
	return NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"}), context.Background()
}

// TestFindSensitiveColumns proves PII-risk columns are found and categorized.
func TestFindSensitiveColumns(t *testing.T) {
	uc, ctx := seedSensitiveSchema(t)

	findings, err := uc.FindSensitiveColumns(ctx, "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	got := map[string]string{} // "table.column" -> category
	for _, f := range findings {
		got[f.Table+"."+f.Column] = f.Category
	}

	expect := map[string]string{
		"customers.email":        "email",
		"customers.phone_number": "phone",
		"customers.ssn":          "national_id",
		"customers.card_token":   "card",
		"customers.full_name":    "personal_name",
		"customers.birth_date":   "date_of_birth",
		"customers.street_addr":  "address",
		"customers.iban_code":    "bank_account",
	}
	for key, want := range expect {
		if got[key] != want {
			t.Errorf("%s: category=%q want %q (all findings: %v)", key, got[key], want, findings)
		}
	}

	for _, benign := range []string{"orders.total", "orders.status", "orders.customer_id", "orders.id"} {
		if _, bad := got[benign]; bad {
			t.Errorf("benign column %s flagged as sensitive", benign)
		}
	}
}

// TestFindSensitiveColumns_NoPII proves empty result on a clean schema.
func TestFindSensitiveColumns_NoPII(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE metrics (id INTEGER, value REAL, label TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	findings, err := uc.FindSensitiveColumns(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings, got %+v", findings)
	}
}

// TestFormatSensitiveColumnsReport proves the rendered report groups by
// table and includes mask_pii guidance.
func TestFormatSensitiveColumnsReport(t *testing.T) {
	uc, ctx := seedSensitiveSchema(t)
	findings, _ := uc.FindSensitiveColumns(ctx, "db1")
	out := FormatSensitiveColumnsReport("db1", findings)
	if !strings.Contains(out, "customers") || !strings.Contains(out, "mask_pii") {
		t.Fatalf("report should group by table and recommend mask_pii:\n%s", out)
	}
}
