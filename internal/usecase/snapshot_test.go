package usecase

import (
	"context"
	"strings"
	"testing"
)

func seedUsers(t *testing.T) (*DatabaseUseCase, context.Context) {
	t.Helper()
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO users (id, name) VALUES (1, 'alice'), (2, 'bob')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	return NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"}), context.Background()
}

// TestDeleteSnapshotAndRollback proves a DELETE captures the removed rows
// and a rollback restores them exactly.
func TestDeleteSnapshotAndRollback(t *testing.T) {
	uc, ctx := seedUsers(t)

	out, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM users WHERE id = 1", nil)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "snapshot") {
		t.Fatalf("expected snapshot reference in result:\n%s", out)
	}

	// Row is gone.
	snapshots := uc.ListSnapshots("db1")
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	restore, err := uc.RollbackSnapshot(ctx, "db1", snapshots[0].ID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !strings.Contains(restore, "restored") && !strings.Contains(restore, "Restored") {
		t.Fatalf("unexpected rollback output:\n%s", restore)
	}

	// Row is back.
	rows, err := uc.ExecuteQueryVerbosity(ctx, "db1", "SELECT name FROM users WHERE id = 1", nil, VerbosityFull)
	if err != nil {
		t.Fatalf("verify query failed: %v", err)
	}
	if !strings.Contains(rows, "alice") {
		t.Fatalf("row not restored:\n%s", rows)
	}
}

// TestUpdateSnapshotRollback proves an UPDATE can be reversed when the table
// carries an id primary-key column.
func TestUpdateSnapshotRollback(t *testing.T) {
	uc, ctx := seedUsers(t)

	if _, err := uc.ExecuteStatement(ctx, "db1", "UPDATE users SET name = 'hacked' WHERE id = 2", nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 || snaps[0].Kind != "update" {
		t.Fatalf("expected update snapshot, got %+v", snaps)
	}

	if _, err := uc.RollbackSnapshot(ctx, "db1", snaps[0].ID); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	rows, _ := uc.ExecuteQueryVerbosity(ctx, "db1", "SELECT name FROM users WHERE id = 2", nil, VerbosityFull)
	if strings.Contains(rows, "hacked") || !strings.Contains(rows, "bob") {
		t.Fatalf("update not reversed:\n%s", rows)
	}
}

// TestSelectCreatesNoSnapshot proves reads never consume snapshot slots.
func TestSelectCreatesNoSnapshot(t *testing.T) {
	uc, ctx := seedUsers(t)
	if _, err := uc.ExecuteQueryVerbosity(ctx, "db1", "SELECT * FROM users", nil, VerbosityFull); err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if _, err := uc.ExecuteStatement(ctx, "db1", "INSERT INTO users (id, name) VALUES (9, 'z')", nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if got := len(uc.ListSnapshots("db1")); got != 0 {
		t.Fatalf("INSERT must not snapshot; got %d", got)
	}
}

// TestRollbackUnknownSnapshotErrors locks in clear failure semantics.
func TestRollbackUnknownSnapshotErrors(t *testing.T) {
	uc, ctx := seedUsers(t)
	if _, err := uc.RollbackSnapshot(ctx, "db1", "snap_999"); err == nil {
		t.Fatal("unknown snapshot id must error")
	}
}

// TestSnapshotRingCap proves bounded retention.
func TestSnapshotRingCap(t *testing.T) {
	uc, ctx := seedUsers(t)
	for i := 100; i < snapshotCapacityPerDB+110; i++ {
		stmt := "DELETE FROM users WHERE id = 2" // deletes 0 or 1 row each pass
		if _, err := uc.ExecuteStatement(ctx, "db1", stmt+" -- "+string(rune('a'+i%26)), nil); err != nil {
			t.Fatalf("delete %d failed: %v", i, err)
		}
		// Re-add so subsequent deletes keep capturing rows.
		if _, err := rawExecSnapSeed(uc, ctx); err != nil {
			t.Fatalf("reseed failed: %v", err)
		}
	}
	if got := len(uc.ListSnapshots("db1")); got != snapshotCapacityPerDB {
		t.Fatalf("ring cap %d exceeded: %d", snapshotCapacityPerDB, got)
	}
}

func rawExecSnapSeed(uc *DatabaseUseCase, ctx context.Context) (string, error) {
	// Re-insert bob if missing; ignore duplicate errors.
	_, err := uc.ExecuteStatement(ctx, "db1", "INSERT OR IGNORE INTO users (id, name) VALUES (2, 'bob')", nil)
	return "", err
}
