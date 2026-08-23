package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestDependencyOrder proves cycle 107: tables render in FK-safe
// topological order (referenced before referencing).
func TestDependencyOrder(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY)`)
	must(`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`)
	must(`CREATE TABLE items (id INTEGER PRIMARY KEY, order_id INTEGER REFERENCES orders(id))`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.DependencyOrder(context.Background(), "db1")
	if err != nil {
		t.Fatalf("order failed: %v", err)
	}
	iUsers := strings.Index(out, "users")
	iOrders := strings.Index(out, "orders")
	iItems := strings.Index(out, "items")
	if iUsers < 0 || iOrders < 0 || iItems < 0 {
		t.Fatalf("tables missing:\n%s", out)
	}
	if !(iUsers < iOrders && iOrders < iItems) {
		t.Fatalf("not topologically ordered:\n%s", out)
	}
}

// TestDependencyOrder_Cycle proves circular FK references are flagged,
// not silently mis-ordered.
func TestDependencyOrder_Cycle(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE a (id INTEGER PRIMARY KEY, b_id INTEGER REFERENCES b(id))`)
	must(`CREATE TABLE b (id INTEGER PRIMARY KEY, a_id INTEGER REFERENCES a(id))`)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.DependencyOrder(context.Background(), "db1")
	if err != nil {
		t.Fatalf("cycle run failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "circular") {
		t.Fatalf("cycle not flagged:\n%s", out)
	}
}

// TestDependencyOrder_Empty proves a database with no FK edges still
// renders every table.
func TestDependencyOrder_Empty(t *testing.T) {
	raw := openSQLiteForTest(t)
	for i, q := range []string{`CREATE TABLE x (id INTEGER PRIMARY KEY)`, `CREATE TABLE y (id INTEGER PRIMARY KEY)`} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("create %d failed: %v", i, err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.DependencyOrder(context.Background(), "db1")
	if err != nil {
		t.Fatalf("empty run failed: %v", err)
	}
	if !strings.Contains(out, "x") || !strings.Contains(out, "y") {
		t.Fatalf("tables missing:\n%s", out)
	}
}
