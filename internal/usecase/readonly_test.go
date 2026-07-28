package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// fakeDB is an in-memory domain.Database for testing the read-only guard.
type fakeDB struct {
	readOnly  bool
	execCalls int
}

func (f *fakeDB) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	return nil, nil
}
func (f *fakeDB) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	f.execCalls++
	return &fakeResult{}, nil
}

type fakeResult struct{}

func (fakeResult) RowsAffected() (int64, error) { return 0, nil }
func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (f *fakeDB) Begin(ctx context.Context, opts *domain.TxOptions) (domain.Tx, error) {
	return nil, nil
}
func (f *fakeDB) IsReadOnly() bool { return f.readOnly }

type fakeRepo struct{ db domain.Database }

func (r *fakeRepo) GetDatabase(_ string) (domain.Database, error) { return r.db, nil }
func (r *fakeRepo) ListDatabases() []string                       { return nil }
func (r *fakeRepo) GetDatabaseType(_ string) (string, error)      { return "", nil }
func (r *fakeRepo) IsLazyLoading() bool                           { return false }

// TestExecuteStatement_ReadOnlyDatabaseRefusesWrites locks in the fix for
// FreePeak/db-mcp-server issue #41: the user asks for a per-database
// read_only flag that prevents write statements (INSERT/UPDATE/DELETE) from
// being executed against production databases. The use-case layer must
// short-circuit Exec when the underlying Database reports IsReadOnly()=true.
func TestExecuteStatement_ReadOnlyDatabaseRefusesWrites(t *testing.T) {
	db := &fakeDB{readOnly: true}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, err := uc.ExecuteStatement(context.Background(), "pg_prod", "DELETE FROM users", nil)
	if err == nil {
		t.Fatal("expected error for write statement on read-only database, got nil")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected error to mention read-only, got: %v", err)
	}
	if db.execCalls != 0 {
		t.Fatalf("Exec must not be called on a read-only database; got %d calls", db.execCalls)
	}
}

// TestExecuteStatement_WritableDatabaseAllowsExec confirms the guard does
// not over-reach: a non-read-only database must still be able to execute
// statements normally.
func TestExecuteStatement_WritableDatabaseAllowsExec(t *testing.T) {
	db := &fakeDB{readOnly: false}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, _ = uc.ExecuteStatement(context.Background(), "pg_dev", "SELECT 1", nil)
	if db.execCalls != 1 {
		t.Fatalf("Exec should be invoked exactly once for a writable database; got %d", db.execCalls)
	}
}
