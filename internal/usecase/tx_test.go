package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
)

// fakeTx is an in-memory domain.Tx recording calls for registry tests.
type fakeTx struct {
	execCalls   int
	queryCalls  int
	commitCalls int
	rollbacks   int
	rollbackErr bool
}

func (t *fakeTx) Commit() error { t.commitCalls++; return nil }
func (t *fakeTx) Rollback() error {
	t.rollbacks++
	if t.rollbackErr {
		return context.Canceled
	}
	return nil
}
func (t *fakeTx) Query(ctx context.Context, query string, args ...interface{}) (domain.Rows, error) {
	t.queryCalls++
	return &fakeRows{columns: []string{"a"}}, nil
}
func (t *fakeTx) Exec(ctx context.Context, statement string, args ...interface{}) (domain.Result, error) {
	t.execCalls++
	return &fakeResult{}, nil
}

// TestExecuteTransaction_BeginStoresRealTransaction locks in the fix for the
// stubbed transaction flow: begin must open a real transaction and register
// it under a fresh transaction ID instead of committing immediately.
func TestExecuteTransaction_BeginStoresRealTransaction(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	msg, meta, err := uc.ExecuteTransaction(context.Background(), "pg_prod", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("unexpected error on begin: %v", err)
	}
	if db.beginCalls != 1 {
		t.Fatalf("expected Begin to be called once, got %d", db.beginCalls)
	}
	if tx.commitCalls != 0 {
		t.Fatal("begin must not commit the transaction")
	}
	id, ok := meta["transactionId"].(string)
	if !ok || id == "" {
		t.Fatalf("expected non-empty transactionId metadata, got %v", meta)
	}
	if !strings.Contains(msg, "started") && !strings.Contains(msg, "Started") {
		t.Fatalf("unexpected begin message: %q", msg)
	}
}

// TestExecuteTransaction_ExecuteUsesStoredTransaction verifies statements run
// inside the registered transaction.
func TestExecuteTransaction_ExecuteUsesStoredTransaction(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, meta, err := uc.ExecuteTransaction(context.Background(), "db", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	txID := meta["transactionId"].(string)

	if _, _, err = uc.ExecuteTransaction(context.Background(), "db", "execute", txID, "INSERT INTO t VALUES (1)", nil, false); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if tx.execCalls != 1 {
		t.Fatalf("expected tx.Exec to be called once, got %d", tx.execCalls)
	}
	if tx.commitCalls != 0 {
		t.Fatal("execute within a transaction must not commit")
	}
}

// TestExecuteTransaction_CommitAndRollbackApplyToRealTransactions verifies
// commit/rollback operate on the stored transaction and retire its ID.
func TestExecuteTransaction_CommitAndRollbackApplyToRealTransactions(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, meta, _ := uc.ExecuteTransaction(context.Background(), "db", "begin", "", "", nil, false)
	txID := meta["transactionId"].(string)

	if _, _, err := uc.ExecuteTransaction(context.Background(), "db", "commit", txID, "", nil, false); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if tx.commitCalls != 1 {
		t.Fatalf("expected one Commit call, got %d", tx.commitCalls)
	}
	// The ID is retired after commit: replaying it must fail loudly instead
	// of silently reporting success (the old stub behavior).
	if _, _, err := uc.ExecuteTransaction(context.Background(), "db", "commit", txID, "", nil, false); err == nil {
		t.Fatal("expected error when committing an already-committed transaction ID")
	}

	// Rollback path with a fresh transaction.
	tx2 := &fakeTx{}
	db2 := &fakeDB{beginTx: tx2}
	uc2 := NewDatabaseUseCase(&fakeRepo{db: db2})
	_, meta2, _ := uc2.ExecuteTransaction(context.Background(), "db", "begin", "", "", nil, false)
	if _, _, err := uc2.ExecuteTransaction(context.Background(), "db", "rollback", meta2["transactionId"].(string), "", nil, false); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if tx2.rollbacks != 1 {
		t.Fatalf("expected one Rollback call, got %d", tx2.rollbacks)
	}
}

// TestExecuteTransaction_UnknownTransactionIDFails ensures unknown IDs error
// clearly rather than faking success.
func TestExecuteTransaction_UnknownTransactionIDFails(t *testing.T) {
	uc := NewDatabaseUseCase(&fakeRepo{db: &fakeDB{}})
	for _, action := range []string{"commit", "rollback", "execute"} {
		if _, _, err := uc.ExecuteTransaction(context.Background(), "db", action, "tx_missing", "SELECT 1", nil, false); err == nil {
			t.Fatalf("expected error for %s with unknown transaction ID", action)
		}
	}
}

// TestExecuteTransaction_ExecuteOnReadOnlyDatabaseBlocksWrites reuses the SQL
// guard for statements executed inside a transaction.
func TestExecuteTransaction_ExecuteOnReadOnlyDatabaseBlocksWrites(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{readOnly: true, beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	// Read-only transactions are allowed to start...
	_, meta, err := uc.ExecuteTransaction(context.Background(), "ro_db", "begin", "", "", nil, true)
	if err != nil {
		t.Fatalf("read-only begin failed: %v", err)
	}
	txID := meta["transactionId"].(string)

	// ...but writes inside them are refused before reaching the driver.
	if _, _, err = uc.ExecuteTransaction(context.Background(), "ro_db", "execute", txID, "DELETE FROM t", nil, true); err == nil {
		t.Fatal("expected write-in-transaction to be refused on read-only database")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got: %v", err)
	}
	if tx.execCalls != 0 {
		t.Fatalf("statement must not reach the transaction; got %d exec calls", tx.execCalls)
	}
}
