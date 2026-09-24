package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/FreePeak/db-mcp-server/internal/domain"
	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func TestDatabaseAdapter_RealSQLiteTransactionSurvivesRequestCancellation(t *testing.T) {
	raw, err := db.NewDatabase(db.Config{
		Type:             "sqlite",
		DatabasePath:     filepath.Join(t.TempDir(), "transactions.db"),
		UseModerncDriver: true,
		QueryTimeout:     2,
	})
	if err != nil {
		t.Fatalf("create sqlite database: %v", err)
	}
	if err := raw.Connect(); err != nil {
		t.Fatalf("connect sqlite database: %v", err)
	}
	defer func() { _ = raw.Close() }()

	adapter := &DatabaseAdapter{db: raw}
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	tx, err := adapter.Begin(requestCtx, &domain.TxOptions{})
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	cancelRequest()

	if _, err := tx.Exec(context.Background(), "CREATE TABLE committed (id INTEGER)"); err != nil {
		t.Fatalf("execute after request cancellation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	rows, err := raw.Query(context.Background(), "SELECT COUNT(*) FROM committed")
	if err != nil {
		t.Fatalf("query committed table: %v", err)
	}
	if !rows.Next() {
		t.Fatal("committed table returned no rows")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan committed count: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close committed query: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty committed table, got %d rows", count)
	}

	rollbackTx, err := adapter.Begin(context.Background(), &domain.TxOptions{})
	if err != nil {
		t.Fatalf("begin rollback transaction: %v", err)
	}
	if _, err := rollbackTx.Exec(context.Background(), "INSERT INTO committed VALUES (1)"); err != nil {
		t.Fatalf("execute rollback transaction: %v", err)
	}
	if err := rollbackTx.Rollback(); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}

	rows2, err := raw.Query(context.Background(), "SELECT COUNT(*) FROM committed")
	if err != nil {
		t.Fatalf("query after rollback: %v", err)
	}
	defer func() { _ = rows2.Close() }()
	if !rows2.Next() {
		t.Fatal("committed table disappeared after rollback")
	}
	if err := rows2.Scan(&count); err != nil {
		t.Fatalf("scan count after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("rollback did not discard the insert; got %d rows", count)
	}
}
