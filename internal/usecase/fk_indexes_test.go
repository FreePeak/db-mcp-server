package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestFindMissingFKIndexes proves cycle 113: foreign-key child columns
// without any index leading on them are flagged with CREATE INDEX DDL;
// indexed FK columns stay silent.
func TestFindMissingFKIndexes(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	must(`CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		coupon_id INTEGER REFERENCES users(id)
	)`)
	must(`CREATE INDEX idx_orders_user ON orders (user_id)`) // covers user_id only
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.FindMissingFKIndexes(context.Background(), "db1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if !strings.Contains(out, "coupon_id") || !strings.Contains(out, "CREATE INDEX") {
		t.Fatalf("unindexed FK not flagged with DDL:\n%s", out)
	}
	if strings.Contains(out, "user_id") && !strings.Contains(out, "coupon_id") &&
		strings.Contains(out, "orders.user_id") {
		t.Fatalf("indexed FK misflagged:\n%s", out)
	}

	// All FKs covered: clean state.
	clean := openSQLiteForTest(t)
	for _, q := range []string{
		`CREATE TABLE p (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE c (id INTEGER PRIMARY KEY, p_id INTEGER REFERENCES p(id))`,
		`CREATE INDEX idx_c_p ON c (p_id)`,
	} {
		if _, err := clean.Exec(q); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: clean}, dbType: "sqlite"})
	out2, err := uc2.FindMissingFKIndexes(context.Background(), "db1")
	if err != nil || !strings.Contains(out2, "No missing") {
		t.Fatalf("clean state wrong (%v):\n%s", err, out2)
	}

	// No FKs at all: clean state too.
	bare := openSQLiteForTest(t)
	if _, err := bare.Exec(`CREATE TABLE lone (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc3 := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: bare}, dbType: "sqlite"})
	out3, err := uc3.FindMissingFKIndexes(context.Background(), "db1")
	if err != nil || !strings.Contains(out3, "No foreign") {
		t.Fatalf("no-fk state wrong (%v):\n%s", err, out3)
	}
}
