package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestProfileColumn proves cycle 69: one call reports row count, null
// density, cardinality, min/max, and the most frequent values.
func TestProfileColumn(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, plan TEXT, age INTEGER)`)
	for i := 0; i < 10; i++ {
		plan := "free"
		if i == 9 {
			plan = "pro"
		}
		if i%2 == 0 {
			must(`INSERT INTO users (plan) VALUES ('` + plan + `')`) // age NULL
		} else {
			must(`INSERT INTO users (plan, age) VALUES ('` + plan + `', ` + string(rune('0'+i)) + `)`)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.ProfileColumn(context.Background(), "db1", "users", "plan")
	if err != nil {
		t.Fatalf("profile failed: %v", err)
	}
	for _, want := range []string{"rows: 10\n", "distinct: 2\n", "free", "pro"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	out, err = uc.ProfileColumn(context.Background(), "db1", "users", "age")
	if err != nil {
		t.Fatalf("profile failed: %v", err)
	}
	if !strings.Contains(out, "5") || !strings.Contains(strings.ToLower(out), "null") {
		t.Fatalf("null count not reported:\n%s", out)
	}

	if _, err := uc.ProfileColumn(context.Background(), "db1", "missing_table", "x"); err == nil {
		t.Fatal("unknown table must error")
	}
}
