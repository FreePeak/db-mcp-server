package usecase

import (
	"context"
	"fmt"
	"testing"
)

// TestDescribeTable_PostgresSchemas_Live checks that describe reads one
// table when another schema has a table with the same name.
// Requires docker-compose.test.yml or any PostgreSQL on port 15432.
func TestDescribeTable_PostgresSchemas_Live(t *testing.T) {
	g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable")
	for _, stmt := range []string{
		"DROP SCHEMA IF EXISTS describe_other CASCADE",
		"DROP TABLE IF EXISTS public.describe_users",
		"CREATE SCHEMA describe_other",
		"CREATE TABLE describe_other.describe_users (id uuid PRIMARY KEY, secret text)",
		"CREATE TABLE public.describe_users (id serial PRIMARY KEY, display_name text)",
	} {
		if _, err := g.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		_, _ = g.db.Exec("DROP SCHEMA IF EXISTS describe_other CASCADE")
		_, _ = g.db.Exec("DROP TABLE IF EXISTS public.describe_users")
	})

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})
	for _, table := range []string{"describe_users", "public.describe_users"} {
		info, err := uc.DescribeTable(context.Background(), "pg1", table)
		if err != nil {
			t.Fatalf("describe %s: %v", table, err)
		}
		var cols []string
		for _, c := range info["columns"].([]map[string]interface{}) {
			cols = append(cols, fmt.Sprint(c["column_name"]))
		}
		if fmt.Sprint(cols) != "[id display_name]" {
			t.Fatalf("describe %s: expected [id display_name], got %v", table, cols)
		}
		if n := len(info["constraints"].([]map[string]interface{})); n != 1 {
			t.Fatalf("describe %s: expected 1 constraint, got %d", table, n)
		}
	}
}

// TestDescribeTable_PostgresMixedCase_Live checks a quoted mixed-case table
// name (Prisma's "User") is still found.
func TestDescribeTable_PostgresMixedCase_Live(t *testing.T) {
	g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable")
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS public."DescribeUser"`,
		`CREATE TABLE public."DescribeUser" (id serial PRIMARY KEY, email text)`,
	} {
		if _, err := g.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() { _, _ = g.db.Exec(`DROP TABLE IF EXISTS public."DescribeUser"`) })

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})
	for _, table := range []string{"DescribeUser", "public.DescribeUser"} {
		info, err := uc.DescribeTable(context.Background(), "pg1", table)
		if err != nil {
			t.Fatalf("describe %s: %v", table, err)
		}
		var cols []string
		for _, c := range info["columns"].([]map[string]interface{}) {
			cols = append(cols, fmt.Sprint(c["column_name"]))
		}
		if fmt.Sprint(cols) != "[id email]" {
			t.Fatalf("describe %s: expected [id email], got %v", table, cols)
		}
	}
}
