package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

func TestIsPlainIdentifier(t *testing.T) {
	valid := []string{"users", "public.users", "_tmp1", "t$1", "v2.orders"}
	invalid := []string{"", "1abc", "users; DROP TABLE x", "a b", "users--x"}
	for _, s := range valid {
		if !isPlainIdentifier(s) {
			t.Errorf("expected %q to be valid identifier", s)
		}
	}
	for _, s := range invalid {
		if isPlainIdentifier(s) {
			t.Errorf("expected %q to be rejected as identifier", s)
		}
	}
}

// TestDescribeTable_EndToEnd describes a real table in in-memory SQLite
// through the full use-case path.
func TestDescribeTable_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT 'anon', price REAL)`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := raw.Exec("INSERT INTO items VALUES (1, 'a', 9.5), (2, 'b', 3.0)"); err != nil {
		t.Fatalf("failed to insert rows: %v", err)
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	info, err := uc.DescribeTable(context.Background(), "sqlite1", "items")
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	columns := info["columns"].([]map[string]interface{})
	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d: %v", len(columns), columns)
	}
	names := map[string]bool{}
	for _, c := range columns {
		if n, ok := c["column_name"].(string); ok {
			names[n] = true
		}
	}
	for _, want := range []string{"id", "name", "price"} {
		if !names[want] {
			t.Fatalf("missing column %q in %v", want, names)
		}
	}
	nullable := map[string]string{}
	for _, c := range columns {
		nullable[fmt.Sprint(c["column_name"])] = fmt.Sprint(c["is_nullable"])
	}
	if nullable["name"] != "NO" || nullable["price"] != "YES" {
		t.Fatalf("expected name NO and price YES, got %v", nullable)
	}

	if rc, ok := info["rowCount"].(string); !ok || rc != "2" {
		t.Fatalf("expected rowCount 2, got %v", info["rowCount"])
	}

	constraints := info["constraints"].([]map[string]interface{})
	types := map[string]bool{}
	for _, c := range constraints {
		if ct, ok := c["constraint_type"].(string); ok {
			types[ct] = true
		}
	}
	if !types["PRIMARY KEY"] {
		t.Fatalf("expected PRIMARY KEY constraint for id column, got %v", constraints)
	}
}

// TestRelationshipGraph_EndToEnd renders a two-table FK schema as Mermaid.
func TestRelationshipGraph_EndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create authors failed: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE books (id INTEGER PRIMARY KEY, author_id INTEGER REFERENCES authors(id), title TEXT)`); err != nil {
		t.Fatalf("create books failed: %v", err)
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	graph, err := uc.RelationshipGraph(context.Background(), "sqlite1")
	if err != nil {
		t.Fatalf("relationship graph failed: %v", err)
	}
	if !strings.Contains(graph, "erDiagram") {
		t.Fatalf("expected erDiagram header, got:\n%s", graph)
	}
	if !strings.Contains(graph, "authors ||--o{ books") || !strings.Contains(graph, `"author_id"`) {
		t.Fatalf("expected authors->books edge, got:\n%s", graph)
	}
}

// TestDescribeTable_ForeignKeyReferenceEndToEnd verifies FK constraints
// resolve to their referenced table and column on a real database.
func TestDescribeTable_ForeignKeyReferenceEndToEnd(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE authors (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create authors failed: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE books (id INTEGER PRIMARY KEY, author_id INTEGER REFERENCES authors(id), title TEXT)`); err != nil {
		t.Fatalf("create books failed: %v", err)
	}

	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	info, err := uc.DescribeTable(context.Background(), "sqlite1", "books")
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	found := false
	constraints := info["constraints"].([]map[string]interface{})
	str := func(m map[string]interface{}, k string) string {
		s, _ := m[k].(string)
		return s
	}
	for _, c := range constraints {
		if str(c, "constraint_type") == "FOREIGN KEY" &&
			str(c, "referenced_table") == "authors" &&
			str(c, "referenced_column") == "id" &&
			str(c, "column_name") == "author_id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected author_id -> authors(id) foreign key, got %v", constraints)
	}
}

// TestDescribeTable_RejectsInjectionInput locks the identifier guard.
func TestDescribeTable_RejectsInjectionInput(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	for _, bad := range []string{"", "users; DROP TABLE users", "users--x", "a b"} {
		if _, err := uc.DescribeTable(context.Background(), "db", bad); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		} else if !strings.Contains(err.Error(), "table parameter") && !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("unexpected error for %q: %v", bad, err)
		}
	}
}

// TestPostgresBuilders_EscapeTableName checks that every Postgres builder
// emits the table name as one escaped SQL literal. IndexHealth passes catalog
// names straight into these builders without isPlainIdentifier, so a quote in
// a name must not end the literal early.
func TestPostgresBuilders_EscapeTableName(t *testing.T) {
	cases := []struct {
		table string
		lit   string
	}{
		{`x' OR '1'='1`, `'x'' OR ''1''=''1'`},
		{`it's`, `'it''s'`},
		{`public.it's`, `'it''s'`},
	}
	for _, tc := range cases {
		queries := map[string]string{
			"columns":     columnQueries("postgres", tc.table)[0],
			"indexes":     indexQueries("postgres", tc.table)[0],
			"constraints": constraintQueries("postgres", tc.table)[0],
			"rowEstimate": rowEstimateQuery("postgres", tc.table),
		}
		for builder, q := range queries {
			if !strings.Contains(q, tc.lit) {
				t.Errorf("%s(%q): expected escaped literal %s in SQL, got:\n%s", builder, tc.table, tc.lit, q)
			}
		}
	}
}

// recordingPGDB records every catalog query and serves one public table name
// for the table-list query, so RelationshipGraph runs its real describe path.
type recordingPGDB struct {
	*fakeDB
	queries []string
}

func (r *recordingPGDB) Query(ctx context.Context, q string, args ...interface{}) (domain.Rows, error) {
	r.queries = append(r.queries, q)
	if strings.Contains(q, "pg_tables") {
		return &fakeRows{columns: []string{"table_name"}, data: [][]interface{}{{"authors"}}}, nil
	}
	return &fakeRows{columns: []string{"column_name"}}, nil
}

// TestRelationshipGraph_PostgresQualifiesPublicTables checks that each listed
// public table is described as public.<name>, so search_path cannot resolve
// the bare name to a same-named table in another schema.
func TestRelationshipGraph_PostgresQualifiesPublicTables(t *testing.T) {
	db := &recordingPGDB{fakeDB: &fakeDB{}}
	uc := NewDatabaseUseCase(&fakeRepo{db: db, dbType: "postgres"})
	if _, err := uc.RelationshipGraph(context.Background(), "pg1"); err != nil {
		t.Fatalf("relationship graph failed: %v", err)
	}
	want := "WHERE table_schema = 'public' AND table_name = 'authors'"
	for _, q := range db.queries {
		if strings.Contains(q, want) {
			return
		}
	}
	t.Fatalf("expected a describe query for public.authors containing %q; recorded queries:\n%s", want, strings.Join(db.queries, "\n---\n"))
}

// mysqlCatalogDB serves SHOW COLUMNS / SHOW INDEX rows in the shape the MySQL
// driver returns them, so DescribeTable runs its real MySQL path.
type mysqlCatalogDB struct {
	*fakeDB
}

func (m *mysqlCatalogDB) Query(ctx context.Context, q string, args ...interface{}) (domain.Rows, error) {
	switch {
	case strings.HasPrefix(q, "SHOW COLUMNS"):
		return &fakeRows{
			columns: []string{"Field", "Type", "Null", "Key", "Default", "Extra"},
			data: [][]interface{}{
				{"id", "bigint unsigned", "NO", "PRI", nil, "auto_increment"},
				{"email", "varchar(255)", "NO", "UNI", nil, ""},
				{"status", "varchar(16)", "YES", "MUL", "active", ""},
			},
		}, nil
	case strings.HasPrefix(q, "SHOW INDEX"):
		cols := []string{"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name", "Sub_part", "Index_type"}
		return &fakeRows{
			columns: cols,
			data: [][]interface{}{
				{"sample_items", "0", "PRIMARY", "1", "id", nil, "BTREE"},
				{"sample_items", "0", "uq_email", "1", "email", nil, "BTREE"},
				// Second key part listed first: definitions follow Seq_in_index.
				{"sample_items", "1", "idx_status_email", "2", "email", "10", "BTREE"},
				{"sample_items", "1", "idx_status_email", "1", "status", nil, "BTREE"},
			},
		}, nil
	}
	return &fakeRows{columns: []string{"n"}}, nil
}

// TestDescribeTable_MySQLNormalizesShowOutput checks that MySQL describe
// returns the column_name / data_type / is_nullable / column_default and
// index_name / definition keys every other engine returns (issue #110).
func TestDescribeTable_MySQLNormalizesShowOutput(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &mysqlCatalogDB{fakeDB: &fakeDB{}}, dbType: "mysql"})
	info, err := uc.DescribeTable(context.Background(), "mysql1", "sample_items")
	if err != nil {
		t.Fatalf("describe failed: %v", err)
	}

	var gotCols []string
	for _, c := range info["columns"].([]map[string]interface{}) {
		gotCols = append(gotCols, fmt.Sprintf("%v|%v|%v|%v", c["column_name"], c["data_type"], c["is_nullable"], c["column_default"]))
	}
	wantCols := []string{
		"id|bigint unsigned|NO|<nil>",
		"email|varchar(255)|NO|<nil>",
		"status|varchar(16)|YES|active",
	}
	if strings.Join(gotCols, "\n") != strings.Join(wantCols, "\n") {
		t.Fatalf("columns:\n got %v\nwant %v", gotCols, wantCols)
	}

	var gotIdx []string
	for _, ix := range info["indexes"].([]map[string]interface{}) {
		gotIdx = append(gotIdx, fmt.Sprintf("%v: %v", ix["index_name"], ix["definition"]))
	}
	wantIdx := []string{
		"PRIMARY: UNIQUE BTREE (id)",
		"uq_email: UNIQUE BTREE (email)",
		"idx_status_email: BTREE (status, email(10))",
	}
	if strings.Join(gotIdx, "\n") != strings.Join(wantIdx, "\n") {
		t.Fatalf("indexes:\n got %v\nwant %v", gotIdx, wantIdx)
	}
}

// TestNormalizeMySQLIndexes_BinaryProtocolValues covers integer cells, as the
// driver returns them for binary-protocol results, and functional key parts.
func TestNormalizeMySQLIndexes_BinaryProtocolValues(t *testing.T) {
	rows := []map[string]interface{}{
		{"Key_name": "idx_lower", "Non_unique": int64(1), "Seq_in_index": int64(1), "Column_name": nil, "Expression": "lower(`email`)", "Index_type": "BTREE"},
		{"Key_name": "PRIMARY", "Non_unique": int64(0), "Seq_in_index": int64(1), "Column_name": []byte("id"), "Index_type": "BTREE"},
	}
	got := normalizeMySQLIndexes(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 indexes, got %v", got)
	}
	if got[0]["definition"] != "BTREE (lower(`email`))" || got[1]["definition"] != "UNIQUE BTREE (id)" {
		t.Fatalf("unexpected definitions: %v", got)
	}
}

// TestMySQLCatalogQueries_ScopeToTable checks that the constraint join matches
// the table name, since every MySQL primary key is named PRIMARY, and that the
// row estimate reads the current database only.
func TestMySQLCatalogQueries_ScopeToTable(t *testing.T) {
	q := constraintQueries("mysql", "sample_items")[0]
	if !strings.Contains(q, "tc.TABLE_NAME = kcu.TABLE_NAME") {
		t.Errorf("constraint join must match table names, got:\n%s", q)
	}
	if !strings.Contains(q, "tc.TABLE_NAME = 'sample_items' AND tc.TABLE_SCHEMA = DATABASE()") {
		t.Errorf("constraint query must filter the requested table, got:\n%s", q)
	}
	if est := rowEstimateQuery("mysql", "sample_items"); !strings.Contains(est, "table_schema = DATABASE() AND table_name = 'sample_items'") {
		t.Errorf("row estimate must read the current database only, got:\n%s", est)
	}
}
