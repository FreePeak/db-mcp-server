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

// TestMaskingAudit_FileSinkPersistsEvents proves redaction events survive
// process restarts when a JSONL sink path is configured.
func TestMaskingAudit_FileSinkPersistsEvents(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t (email) VALUES ('a@b.c')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if err := uc.EnableMaskingAuditFile(path); err != nil {
		t.Fatalf("enable sink failed: %v", err)
	}

	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM t", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if err := uc.CloseMaskingAuditFile(); err != nil {
		t.Fatalf("close sink failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("audit file missing: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var ev MaskingAuditEvent
	found := false
	for scanner.Scan() {
		line := scanner.Bytes()
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", line, err)
		}
		found = true
	}
	if !found {
		t.Fatal("no events persisted to sink")
	}
	if ev.DatabaseID != "db1" || ev.CellsMasked != 1 || !strings.Contains(ev.Query, "SELECT email") {
		t.Fatalf("unexpected persisted event: %+v", ev)
	}
}

// TestMaskingAudit_FileSinkAppendMode proves an existing audit file is
// appended to, never truncated.
func TestMaskingAudit_FileSinkAppendMode(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO t (email) VALUES ('a@b.c')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(`{"seed":true}`+"\n"), 0o600); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if err := uc.EnableMaskingAuditFile(path); err != nil {
		t.Fatalf("enable sink failed: %v", err)
	}
	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM t", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if err := uc.CloseMaskingAuditFile(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Count(string(data), "\n")
	if lines < 2 {
		t.Fatalf("expected seed line preserved + new event, got %d lines", lines)
	}
	if !strings.Contains(string(data), `"seed":true`) {
		t.Fatal("append mode violated: seed content lost")
	}
}

// TestMaskingAudit_NoSinkInMemoryOnly proves absence of a configured file
// keeps behavior identical (in-memory only, no stray files).
func TestMaskingAudit_NoSinkInMemoryOnly(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	if _, err := uc.ExecuteQueryMasked(context.Background(), "db1", "SELECT email FROM t", nil, true); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if uc.maskingAudit.sinkPath() != "" {
		t.Fatalf("expected no sink path by default, got %q", uc.maskingAudit.sinkPath())
	}
}
