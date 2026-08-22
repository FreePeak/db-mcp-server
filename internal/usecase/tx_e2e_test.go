package usecase

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// sqliteDB adapts a real *sql.DB to domain.Database for end-to-end
// transaction tests against an in-memory SQLite database.
type sqliteDB struct {
	db *sql.DB
}

func (s *sqliteDB) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}
func (s *sqliteDB) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	return s.db.ExecContext(ctx, statement, args...)
}
func (s *sqliteDB) Begin(ctx context.Context, opts *domain.TxOptions) (domain.Tx, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: opts.ReadOnly})
	if err != nil {
		return nil, err
	}
	return &sqliteTx{tx: tx}, nil
}
func (s *sqliteDB) IsReadOnly() bool { return false }
func (s *sqliteDB) MaxRows() int     { return 0 }

type sqliteTx struct{ tx *sql.Tx }

func (t *sqliteTx) Commit() error   { return t.tx.Commit() }
func (t *sqliteTx) Rollback() error { return t.tx.Rollback() }
func (t *sqliteTx) Query(ctx context.Context, q string, args ...interface{}) (domain.Rows, error) {
	return t.tx.QueryContext(ctx, q, args...)
}
func (t *sqliteTx) Exec(ctx context.Context, q string, args ...interface{}) (domain.Result, error) {
	return t.tx.ExecContext(ctx, q, args...)
}

// TestExecuteTransaction_EndToEndRollbackAndCommit proves multi-statement
// transactions behave correctly through the full use-case path: a rollback
// discards staged writes, a commit persists them. The previous stub behavior
// committed at begin and faked the rest, silently corrupting expectations.
func TestExecuteTransaction_EndToEndRollbackAndCommit(t *testing.T) {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer func() { _ = raw.Close() }()

	if _, err := raw.Exec("CREATE TABLE kv (k TEXT PRIMARY KEY, v INTEGER)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper})
	ctx := context.Background()

	beginTx := func(t *testing.T) string {
		t.Helper()
		_, meta, err := uc.ExecuteTransaction(ctx, "sqlite1", "begin", "", "", nil, false)
		if err != nil {
			t.Fatalf("begin failed: %v", err)
		}
		id, ok := meta["transactionId"].(string)
		if !ok || id == "" {
			t.Fatalf("missing transactionId in metadata: %v", meta)
		}
		return id
	}
	execInTx := func(t *testing.T, txID, stmt string) {
		t.Helper()
		if _, _, err := uc.ExecuteTransaction(ctx, "sqlite1", "execute", txID, stmt, nil, false); err != nil {
			t.Fatalf("execute %q failed: %v", stmt, err)
		}
	}
	countRows := func(t *testing.T) int {
		t.Helper()
		out, err := uc.ExecuteQuery(ctx, "sqlite1", "SELECT COUNT(*) AS n FROM kv", nil)
		if err != nil {
			t.Fatalf("count query failed: %v", err)
		}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if n, err := strconv.Atoi(line); err == nil {
				return n
			}
		}
		t.Fatalf("failed to parse count from output %q", out)
		return -1
	}

	// Rolled-back writes must leave no trace.
	txID := beginTx(t)
	execInTx(t, txID, "INSERT INTO kv VALUES ('rolled_back', 1)")
	execInTx(t, txID, "INSERT INTO kv VALUES ('also_gone', 2)")
	if _, _, err := uc.ExecuteTransaction(ctx, "sqlite1", "rollback", txID, "", nil, false); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if got := countRows(t); got != 0 {
		t.Fatalf("rollback did not discard writes; expected 0 rows, found %d", got)
	}

	// Committed writes must persist.
	txID2 := beginTx(t)
	execInTx(t, txID2, "INSERT INTO kv VALUES ('kept', 42)")
	if _, _, err := uc.ExecuteTransaction(ctx, "sqlite1", "commit", txID2, "", nil, false); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if got := countRows(t); got != 1 {
		t.Fatalf("commit did not persist writes; expected 1 row, found %d", got)
	}
}
