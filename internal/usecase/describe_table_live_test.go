package usecase

import (
	"context"
	"fmt"
	"strings"
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

// TestPostgresQuoteInTableName_Live checks that a catalog table name holding a
// single quote reaches the Postgres catalog builders escaped. DescribeTable
// rejects such names by design, but IndexHealth passes catalog names straight
// through, so its duplicate-index finding is the end-to-end check.
func TestPostgresQuoteInTableName_Live(t *testing.T) {
	g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable")
	for _, stmt := range []string{
		`DROP TABLE IF EXISTS public."it's" CASCADE`,
		`CREATE TABLE public."it's" (id serial PRIMARY KEY, note text)`,
		`CREATE INDEX "it's_note_a" ON public."it's" (note)`,
		`CREATE INDEX "it's_note_b" ON public."it's" (note)`,
	} {
		if _, err := g.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() { _, _ = g.db.Exec(`DROP TABLE IF EXISTS public."it's" CASCADE`) })

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})
	rows, err := uc.queryTableMetadata(context.Background(), "pg1", columnQueries("postgres", "it's"))
	if err != nil {
		t.Fatalf("column query for it's: %v", err)
	}
	var cols []string
	for _, r := range rows {
		cols = append(cols, fmt.Sprint(r["column_name"]))
	}
	if fmt.Sprint(cols) != "[id note]" {
		t.Fatalf("expected columns [id note] for it's, got %v", cols)
	}

	report, err := uc.IndexHealth(context.Background(), "pg1")
	if err != nil {
		t.Fatalf("index health: %v", err)
	}
	if !strings.Contains(report, `DUPLICATE on it's`) {
		t.Fatalf("expected duplicate-index finding for it's, got:\n%s", report)
	}
}

// TestRelationshipGraph_PostgresSchemas_Live checks that the graph describes
// public.<name>. The connection's search_path lists describe_other first, so a
// bare name would resolve to describe_other.describe_books and pick up its FK.
func TestRelationshipGraph_PostgresSchemas_Live(t *testing.T) {
	g := openLive(t, "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable search_path=describe_other,public")
	for _, stmt := range []string{
		"DROP SCHEMA IF EXISTS describe_other CASCADE",
		"DROP TABLE IF EXISTS public.describe_books, public.describe_authors",
		"CREATE SCHEMA describe_other",
		"CREATE TABLE describe_other.describe_owners (id uuid PRIMARY KEY)",
		"CREATE TABLE describe_other.describe_books (id uuid PRIMARY KEY, owner_id uuid REFERENCES describe_other.describe_owners(id))",
		"CREATE TABLE public.describe_authors (id int PRIMARY KEY)",
		"CREATE TABLE public.describe_books (id int PRIMARY KEY, author_id int REFERENCES public.describe_authors(id))",
	} {
		if _, err := g.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		_, _ = g.db.Exec("DROP SCHEMA IF EXISTS describe_other CASCADE")
		_, _ = g.db.Exec("DROP TABLE IF EXISTS public.describe_books, public.describe_authors")
	})

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "postgres"})
	graph, err := uc.RelationshipGraph(context.Background(), "pg1")
	if err != nil {
		t.Fatalf("relationship graph: %v", err)
	}
	if !strings.Contains(graph, `describe_authors ||--o{ describe_books : "author_id"`) {
		t.Fatalf("expected public.describe_books -> describe_authors edge, got:\n%s", graph)
	}
	if strings.Contains(graph, "describe_owners") {
		t.Fatalf("graph resolved describe_books to describe_other, got:\n%s", graph)
	}
}

// TestDescribeTable_MySQL_Live checks that describe returns named columns and
// indexes and only this table's primary key when other tables have one too.
// Requires docker-compose.test.yml or any MySQL on port 13306.
func TestDescribeTable_MySQL_Live(t *testing.T) {
	g := openLive(t, "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true")
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS describe_items, describe_other",
		"CREATE TABLE describe_other (other_id INT PRIMARY KEY)",
		"CREATE TABLE describe_items (id INT PRIMARY KEY, email VARCHAR(255) NOT NULL, status VARCHAR(16) DEFAULT 'active', UNIQUE KEY uq_email (email), KEY idx_status (status, email))",
	} {
		if _, err := g.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() { _, _ = g.db.Exec("DROP TABLE IF EXISTS describe_items, describe_other") })

	uc := NewDatabaseUseCase(&fakeRepo{db: g, dbType: "mysql"})
	info, err := uc.DescribeTable(context.Background(), "mysql1", "describe_items")
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var cols []string
	for _, c := range info["columns"].([]map[string]interface{}) {
		cols = append(cols, fmt.Sprintf("%v %v", c["column_name"], c["data_type"]))
	}
	if strings.Join(cols, ",") != "id int,email varchar(255),status varchar(16)" {
		t.Fatalf("unexpected columns: %v", cols)
	}
	idx := map[string]string{}
	for _, ix := range info["indexes"].([]map[string]interface{}) {
		idx[fmt.Sprint(ix["index_name"])] = fmt.Sprint(ix["definition"])
	}
	if idx["PRIMARY"] != "UNIQUE BTREE (id)" || idx["idx_status"] != "BTREE (status, email)" {
		t.Fatalf("unexpected indexes: %v", idx)
	}
	for _, c := range info["constraints"].([]map[string]interface{}) {
		if fmt.Sprint(c["column_name"]) == "other_id" {
			t.Fatalf("constraints include another table's primary key: %v", info["constraints"])
		}
	}
}
