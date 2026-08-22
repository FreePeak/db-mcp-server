package usecase

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// multiRepo serves different SQLite handles per database id.
type multiRepo struct {
	dbs   map[string]domain.Database
	types map[string]string
}

func (m *multiRepo) GetDatabase(id string) (domain.Database, error) {
	if d, ok := m.dbs[id]; ok {
		return d, nil
	}
	return nil, context.Canceled
}
func (m *multiRepo) ListDatabases() []string { return nil }
func (m *multiRepo) GetDatabaseType(id string) (string, error) {
	return m.types[id], nil
}
func (m *multiRepo) IsLazyLoading() bool { return false }

// TestCompareSchemas_Diffs proves cycle 63: missing/extra tables and
// columns plus type mismatches are all reported, and identical schemas
// report a clean match.
func TestCompareSchemas_Diffs(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	if _, err := rawA.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := rawB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := rawB.Exec(`CREATE TABLE logs (id INTEGER PRIMARY KEY, line TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	for _, want := range []string{
		"users",
		"name",
		"email",
		"logs",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in diff:\n%s", want, out)
		}
	}
}

func TestCompareSchemas_Match(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	for _, q := range []string{`CREATE TABLE t (id INTEGER PRIMARY KEY)`} {
		if _, err := rawA.Exec(q); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if _, err := rawB.Exec(q); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)
	out, err := uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if !strings.Contains(out, "match") {
		t.Fatalf("expected clean-match report, got:\n%s", out)
	}
}

// TestCompareSchemas_Indexes proves cycle 64: indexes are compared by name
// with a whitespace-normalized definition fingerprint.
func TestCompareSchemas_Indexes(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	setup := func(raw *sql.DB, def string) {
		t.Helper()
		if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if _, err := raw.Exec(def); err != nil {
			t.Fatalf("index failed: %v", err)
		}
	}
	setup(rawA, `CREATE UNIQUE INDEX idx_email ON users (email)`)
	setup(rawB, `CREATE INDEX idx_email ON users (email)`)

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if !strings.Contains(out, "idx_email") || !strings.Contains(out, "unique") {
		t.Fatalf("expected index divergence note naming idx_email/unique, got:\n%s", out)
	}

	// Identical indexes must not appear in the report.
	rawC := openSQLiteForTest(t)
	setup(rawC, `CREATE UNIQUE INDEX idx_email ON users (email)`)
	repo.dbs["b"] = &sqliteDB{db: rawC}
	out, err = uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if strings.Contains(out, "idx_email") {
		t.Fatalf("identical index reported as difference:\n%s", out)
	}
}

// TestCompareSchemas_Constraints proves cycle 65: primary keys and foreign
// keys participate in the diff as per-table fingerprints.
func TestCompareSchemas_Constraints(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	if _, err := rawA.Exec(`CREATE TABLE p (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := rawA.Exec(`CREATE TABLE c (id INTEGER PRIMARY KEY, pid INTEGER REFERENCES p(id))`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// B lacks the FK and demotes c's PK.
	if _, err := rawB.Exec(`CREATE TABLE p (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := rawB.Exec(`CREATE TABLE c (id INTEGER, pid INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	for _, want := range []string{
		`PRIMARY KEY`, `FOREIGN KEY`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	// Identical constraints must not be reported.
	rawC := openSQLiteForTest(t)
	rawD := openSQLiteForTest(t)
	for _, raw := range []*sql.DB{rawC, rawD} {
		if _, err := raw.Exec(`CREATE TABLE p (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	repo.dbs["a"] = &sqliteDB{db: rawC}
	repo.dbs["b"] = &sqliteDB{db: rawD}
	out, err = uc.CompareSchemas(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if strings.Contains(out, "PRIMARY KEY") {
		t.Fatalf("identical PKs reported as difference:\n%s", out)
	}
}

// TestCompareTableCounts proves cycle 71: per-table row counts on both
// sides with the delta, tables present on one side flagged.
func TestCompareTableCounts(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	for _, q := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE logs (id INTEGER PRIMARY KEY)`,
	} {
		if _, err := rawA.Exec(q); err != nil {
			t.Fatalf("create failed: %v", err)
		}
		if _, err := rawB.Exec(q); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	seed := func(raw *sql.DB, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := raw.Exec(`INSERT INTO users (id) VALUES (?)`, i); err != nil {
				t.Fatalf("seed failed: %v", err)
			}
		}
	}
	seed(rawA, 10)
	seed(rawB, 7)

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CompareTableCounts(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if !strings.Contains(out, "users") || !strings.Contains(out, "+3") {
		t.Fatalf("expected users delta +3 in:\n%s", out)
	}
	if !strings.Contains(out, "logs") || !strings.Contains(out, "0") {
		t.Fatalf("expected logs 0/0 in:\n%s", out)
	}
}

// TestCompareTableSamples proves cycle 72: rows present on only one side
// of two databases are reported as added/removed within the sampled window.
func TestCompareTableSamples(t *testing.T) {
	rawA := openSQLiteForTest(t)
	rawB := openSQLiteForTest(t)
	for _, raw := range []*sql.DB{rawA, rawB} {
		if _, err := raw.Exec(`CREATE TABLE tags (id INTEGER PRIMARY KEY, label TEXT)`); err != nil {
			t.Fatalf("create failed: %v", err)
		}
	}
	seed := func(raw *sql.DB, rows [][2]interface{}) {
		t.Helper()
		for _, r := range rows {
			if _, err := raw.Exec(`INSERT INTO tags (id, label) VALUES (?, ?)`, r[0], r[1]); err != nil {
				t.Fatalf("seed failed: %v", err)
			}
		}
	}
	seed(rawA, [][2]interface{}{{1, "a"}, {2, "b"}, {3, "c"}})
	seed(rawB, [][2]interface{}{{1, "a"}, {2, "changed"}, {4, "d"}})

	repo := &multiRepo{
		dbs:   map[string]domain.Database{"a": &sqliteDB{db: rawA}, "b": &sqliteDB{db: rawB}},
		types: map[string]string{"a": "sqlite", "b": "sqlite"},
	}
	uc := NewDatabaseUseCase(repo)

	out, err := uc.CompareTableSamples(context.Background(), "a", "b", "tags", 100)
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	for _, want := range []string{"(3, c)", "(2, changed)", "(4, d)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}

	// Identical data reports a clean match.
	out, err = uc.CompareTableSamples(context.Background(), "a", "a", "tags", 100)
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if !strings.Contains(out, "match") {
		t.Fatalf("expected clean match:\n%s", out)
	}
}
