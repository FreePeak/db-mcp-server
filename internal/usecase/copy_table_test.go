package usecase

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// TestCopyTable proves cycle 96: rows move between two databases in one
// call; conflicts surface as a clear failure naming the target.
func TestCopyTable(t *testing.T) {
	rawSrc := openSQLiteForTest(t)
	rawDst := openSQLiteForTest(t)
	for _, raw := range []*sql.DB{rawSrc, rawDst} {
		if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, label TEXT)`); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	if _, err := rawSrc.Exec(`INSERT INTO items VALUES (1, 'a'), (2, 'b'), (3, NULL)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	repo := &multiRepo{
		dbs:   map[string]domain.Database{"src": &sqliteDB{db: rawSrc}, "dst": &sqliteDB{db: rawDst}},
		types: map[string]string{"src": "sqlite", "dst": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CopyTable(context.Background(), "src", "dst", "items")
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if !strings.Contains(out, "3") || !strings.Contains(out, "items") {
		t.Fatalf("expected 3 copied:\n%s", out)
	}
	var n int
	if err := rawDst.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("destination rows = %d err=%v, want 3", n, err)
	}

	// Re-copy hits the PK conflict and names the target database/table.
	_, err = uc.CopyTable(context.Background(), "src", "dst", "items")
	if err == nil {
		t.Fatal("duplicate copy must fail")
	}
	if !strings.Contains(err.Error(), "dst") || !strings.Contains(err.Error(), "items") {
		t.Fatalf("error should name destination and table:\n%v", err)
	}

	// Missing target table fails clearly.
	if _, err := uc.CopyTable(context.Background(), "src", "dst", "ghost"); err == nil {
		t.Fatal("missing target table must fail")
	}
}
