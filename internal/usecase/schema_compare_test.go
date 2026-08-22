package usecase

import (
	"context"
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
