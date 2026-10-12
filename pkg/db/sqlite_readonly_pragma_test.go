package db

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression coverage for the SQLite write-classification gap the PRD flagged:
// read_only SQLite opens with mode=ro, but mode=ro is not a blanket refusal for
// PRAGMA. Measured against the driver this server uses:
//
//	PRAGMA user_version=5        -> rejected ("attempt to write a readonly database")
//	PRAGMA journal_mode=WAL      -> rejected
//	PRAGMA incremental_vacuum    -> rejected
//	PRAGMA writable_schema=ON    -> ACCEPTED (no error, no persistence)
//	PRAGMA optimize              -> ACCEPTED (no error, no persistence)
//	PRAGMA auto_vacuum=FULL      -> ACCEPTED (no error, no persistence)
//	PRAGMA shrink_memory         -> ACCEPTED (no error, no persistence)
//	PRAGMA wal_checkpoint(TRUNC) -> ACCEPTED (no error, no persistence)
//
// So the guardrail holds — nothing reaches disk — but a handful of mutating
// PRAGMAs return success anyway, because the engine accepts them in memory and
// only the write-back fails silently. This test pins the property that matters:
// after every one of them runs through a read-only connection, the database
// file is byte-identical.
func TestSQLiteReadOnly_MutatingPragmasDoNotPersist(t *testing.T) {
	initLoggerForTests()

	path := filepath.Join(t.TempDir(), "readonly.db")
	ctx := context.Background()

	writable, err := NewDatabase(Config{Type: "sqlite", DatabasePath: path, UseModerncDriver: true})
	if err != nil {
		t.Fatalf("failed to create writable database: %v", err)
	}
	if err := writable.Connect(); err != nil {
		t.Fatalf("failed to connect writable database: %v", err)
	}
	if _, err := writable.Exec(ctx, "CREATE TABLE widget (id INTEGER PRIMARY KEY, label TEXT)"); err != nil {
		t.Fatalf("failed to create fixture table: %v", err)
	}
	if _, err := writable.Exec(ctx, "INSERT INTO widget (label) VALUES ('alpha')"); err != nil {
		t.Fatalf("failed to seed fixture row: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("failed to close writable database: %v", err)
	}

	dsn := buildSQLiteConnStr(Config{Type: "sqlite", DatabasePath: path, ReadOnly: true})
	if !strings.Contains(dsn, "mode=ro") {
		t.Fatalf("read-only SQLite must open mode=ro, got %q", dsn)
	}

	ro, err := NewDatabase(Config{Type: "sqlite", DatabasePath: path, ReadOnly: true, UseModerncDriver: true})
	if err != nil {
		t.Fatalf("failed to create read-only database: %v", err)
	}
	if err := ro.Connect(); err != nil {
		t.Fatalf("failed to connect read-only database: %v", err)
	}
	defer func() { _ = ro.Close() }()

	// Reads must still work on the read-only connection.
	rows, err := ro.Query(ctx, "PRAGMA table_info(widget)")
	if err != nil {
		t.Fatalf("read-only connection must still read: %v", err)
	}
	_ = rows.Close()

	for _, stmt := range []string{
		"PRAGMA user_version=99",
		"PRAGMA schema_version=99",
		"PRAGMA application_id=1234",
		"PRAGMA journal_mode=WAL",
		"PRAGMA writable_schema=ON",
		"PRAGMA auto_vacuum=FULL",
		"PRAGMA optimize",
		"PRAGMA incremental_vacuum",
		"PRAGMA shrink_memory",
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"PRAGMA analyze",
		"PRAGMA vacuum",
	} {
		t.Run(stmt, func(t *testing.T) {
			before := hashFile(t, path)

			// Either the engine refuses it (preferred) or it accepts it in
			// memory; both are fine as long as nothing lands on disk.
			if _, execErr := ro.Exec(ctx, stmt); execErr != nil {
				t.Logf("engine refused %q: %v", stmt, execErr)
			} else {
				t.Logf("engine accepted %q; asserting no persistence", stmt)
			}

			if after := hashFile(t, path); after != before {
				t.Fatalf("%q changed the database file through a read-only connection", stmt)
			}
		})
	}
}

func hashFile(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return sha256.Sum256(data)
}
