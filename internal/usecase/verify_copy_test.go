package usecase

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// TestVerifyCopy proves cycle 99: row-count reconciliation between source
// and destination after a copy, matching and mismatching alike.
func TestVerifyCopy(t *testing.T) {
	rawSrc := openSQLiteForTest(t)
	rawDst := openSQLiteForTest(t)
	for _, raw := range []*sql.DB{rawSrc, rawDst} {
		if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	if _, err := rawSrc.Exec(`INSERT INTO items VALUES (1), (2), (3)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if _, err := rawDst.Exec(`INSERT INTO items VALUES (1), (2)`); err != nil {
		t.Fatalf("seed dst failed: %v", err)
	}
	repo := &multiRepo{
		dbs:   map[string]domain.Database{"src": &sqliteDB{db: rawSrc}, "dst": &sqliteDB{db: rawDst}},
		types: map[string]string{"src": "sqlite", "dst": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.VerifyCopy(context.Background(), "src", "dst", "items")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !strings.Contains(out, "MISMATCH") || !strings.Contains(out, "3") || !strings.Contains(out, "2") {
		t.Fatalf("mismatch not reported:\n%s", out)
	}

	// Matching counts reconcile.
	if _, err := rawDst.Exec(`INSERT INTO items VALUES (3)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	out, err = uc.VerifyCopy(context.Background(), "src", "dst", "items")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !strings.Contains(out, "match") {
		t.Fatalf("expected match:\n%s", out)
	}

	// Missing table fails clearly.
	if _, err := uc.VerifyCopy(context.Background(), "src", "dst", "ghost"); err == nil {
		t.Fatal("missing table must fail")
	}
}
