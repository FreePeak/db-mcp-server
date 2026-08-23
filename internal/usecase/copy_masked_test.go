package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// TestCopyTableMasked proves cycle 117: an anonymized cross-database
// copy replaces PII-bearing values with masked text while non-PII
// values arrive intact.
func TestCopyTableMasked(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, phone TEXT, name TEXT)`,
	} {
		if _, err := rawA.Exec(q); err != nil {
			t.Fatalf("setup A failed: %v", err)
		}
		if _, err := rawB.Exec(q); err != nil {
			t.Fatalf("setup B failed: %v", err)
		}
	}
	if _, err := rawA.Exec(`INSERT INTO users VALUES (1, 'jane@corp.io', '+1-555-234-5678', 'Jane Doe')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CopyTableMasked(context.Background(), "a", "b", "users")
	if err != nil {
		t.Fatalf("masked copy failed: %v", err)
	}
	if !strings.Contains(out, "Copied 1 row") || !strings.Contains(out, "anonymized") {
		t.Fatalf("unexpected report:\n%s", out)
	}

	rows, err := rawB.Query(`SELECT email, phone, name FROM users`)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no destination row")
	}
	var email, phone, name string
	if err := rows.Scan(&email, &phone, &name); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if strings.Contains(email, "jane@corp.io") {
		t.Fatalf("email not masked in destination: %q", email)
	}
	if strings.Contains(phone, "555-234-5678") {
		t.Fatalf("phone not masked in destination: %q", phone)
	}
	if name != "Jane Doe" {
		t.Fatalf("non-PII value altered: %q", name)
	}
}
