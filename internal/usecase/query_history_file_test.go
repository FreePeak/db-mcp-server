package usecase

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQueryHistory_FileSinkPersistsEvents proves history entries survive
// process restarts when a JSONL sink is configured.
func TestQueryHistory_FileSinkPersistsEvents(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	path := filepath.Join(t.TempDir(), "history.jsonl")
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if err := uc.EnableQueryHistoryFile(path); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	if _, err := uc.ExecuteStatement(context.Background(), "db1", "INSERT INTO t VALUES (1)", nil); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if err := uc.CloseQueryHistoryFile(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("sink file missing: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	found := false
	for scanner.Scan() {
		var e HistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("invalid JSONL %q: %v", scanner.Bytes(), err)
		}
		if strings.Contains(e.Statement, "INSERT") {
			found = true
		}
	}
	if !found {
		t.Fatal("no INSERT entry persisted")
	}
}

// TestQueryHistory_SinkErrorsNeverBreakExecution proves a failing sink does
// not affect query serving.
func TestQueryHistory_SinkErrorsNeverBreakExecution(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// Point the sink at a directory — open must fail fast with a clear error
	// (configuration-time feedback beats silent runtime write failures).
	dir := t.TempDir()
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if err := uc.EnableQueryHistoryFile(dir); err == nil {
		t.Fatal("expected open failure for directory sink")
	}
	// With no active sink, execution continues unaffected:
	if _, err := uc.ExecuteStatement(context.Background(), "db1", "INSERT INTO t VALUES (2)", nil); err != nil {
		t.Fatalf("execution broken by failed sink setup: %v", err)
	}
}
