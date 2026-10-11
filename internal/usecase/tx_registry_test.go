package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteTransaction_RejectsTransactionFromAnotherDatabase(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, meta, err := uc.ExecuteTransaction(context.Background(), "db_a", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	txID := meta["transactionId"].(string)

	_, _, err = uc.ExecuteTransaction(context.Background(), "db_b", "execute", txID, "INSERT INTO t VALUES (1)", nil, false)
	if err == nil || !strings.Contains(err.Error(), "belongs to database") {
		t.Fatalf("expected database ownership error, got %v", err)
	}
	if tx.execCalls != 0 {
		t.Fatalf("transaction statement reached the wrong database transaction: %d calls", tx.execCalls)
	}
}

func TestExpireTxRollsBackAndRetiresTransaction(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{beginTx: tx}
	uc := NewDatabaseUseCase(&fakeRepo{db: db})

	_, meta, err := uc.ExecuteTransaction(context.Background(), "db", "begin", "", "", nil, false)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	txID := meta["transactionId"].(string)

	uc.txMu.Lock()
	record := uc.transactions[txID]
	generation := record.generation
	uc.txMu.Unlock()
	uc.expireTx(txID, generation)

	if tx.rollbacks != 1 {
		t.Fatalf("expected expiry to roll back once, got %d", tx.rollbacks)
	}
	_, _, err = uc.ExecuteTransaction(context.Background(), "db", "execute", txID, "SELECT 1", nil, false)
	if err == nil || !strings.Contains(err.Error(), "unknown transaction ID") {
		t.Fatalf("expected expired transaction to be unknown, got %v", err)
	}
}
