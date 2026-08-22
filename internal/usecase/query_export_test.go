package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestExecuteQueryFormat_CSV proves cycle 60: format=csv returns RFC4180
// output with header row, comma quoting, and no tabular decoration.
func TestExecuteQueryFormat_CSV(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL)`)
	must(`INSERT INTO items (name, price) VALUES ('plain', 1.5)`)
	must(`INSERT INTO items (name, price) VALUES ('has,comma "and" quote', 2)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.ExecuteQueryFormat(context.Background(), "db1", "SELECT id, name, price FROM items ORDER BY id", nil, "csv")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	want := "id,name,price\n1,plain,1.5\n2,\"has,comma \"\"and\"\" quote\",2\n"
	if out != want {
		t.Fatalf("csv mismatch:\ngot:  %q\nwant: %q", out, want)
	}
}

// TestExecuteQueryFormat_JSON proves format=json emits an object per row.
func TestExecuteQueryFormat_JSON(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t (name) VALUES ('a'), ('b')`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.ExecuteQueryFormat(context.Background(), "db1", "SELECT id, name FROM t ORDER BY id", nil, "json")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	for _, want := range []string{
		`[{"id":1,"name":"a"},`,
		`{"id":2,"name":"b"}]`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestExecuteQueryFormat_Errors(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ExecuteQueryFormat(context.Background(), "db1", "SELECT id FROM t", nil, "xml"); err == nil {
		t.Fatal("unknown format must error")
	}
	if _, err := uc.ExecuteQueryFormat(context.Background(), "db1", "SELECT id FROM t", nil, ""); err != nil {
		t.Fatalf("empty format must default to csv, got: %v", err)
	}
}

// TestExecuteQueryFormat_Inserts proves cycle 66: format=inserts renders
// each row as an INSERT INTO statement for the queried table.
func TestExecuteQueryFormat_Inserts(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL)`)
	must(`INSERT INTO items (name, price) VALUES ('plain', 1.5)`)
	must(`INSERT INTO items (name, price) VALUES ('it''s "quoted"', NULL)`)

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	out, err := uc.ExecuteQueryFormat(context.Background(), "db1",
		"SELECT id, name, price FROM items ORDER BY id", nil, "inserts")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	want := "INSERT INTO items (id, name, price) VALUES (1, 'plain', 1.5);\n" +
		"INSERT INTO items (id, name, price) VALUES (2, 'it''s \"quoted\"', NULL);\n"
	if out != want {
		t.Fatalf("inserts mismatch:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestExecuteQueryFormat_InsertsErrors(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.ExecuteQueryFormat(context.Background(), "db1", "SELECT 1", nil, "inserts"); err == nil {
		t.Fatal("inserts without a FROM table must error")
	}
}

// TestCountQueryRows proves cycle 68: count_only wraps the statement in
// COUNT(*) so an agent can price a SELECT before fetching rows.
func TestCountQueryRows(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE nums (n INTEGER)`)
	for i := 1; i <= 7; i++ {
		must(`INSERT INTO nums (n) VALUES (` + fmt.Sprint(i) + `)`)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.CountQueryRows(context.Background(), "db1", "SELECT n FROM nums WHERE n > 2", nil)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if !strings.Contains(out, "5") {
		t.Fatalf("expected count 5 in:\n%s", out)
	}

	if _, err := uc.CountQueryRows(context.Background(), "db1", "DELETE FROM nums", nil); err == nil {
		t.Fatal("non-SELECT must be rejected")
	}
}
