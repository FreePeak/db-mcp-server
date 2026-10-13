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
	return NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"}), context.Background()
}

func countRows(t *testing.T, uc *DatabaseUseCase, query string) string {
	t.Helper()
	out, err := uc.ExecuteQuery(context.Background(), "db1", query, nil)
	if err != nil {
		t.Fatalf("verification query failed: %v", err)
	}
	return out
}

// TestDeleteSnapshotAndRollback: a DELETE captures the removed rows and a
// rollback puts them back exactly.
func TestDeleteSnapshotAndRollback(t *testing.T) {
	uc, ctx := seedUsers(t)

	out, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM users WHERE id = 1", nil)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "snapshot") {
		t.Fatalf("expected a snapshot reference in the result:\n%s", out)
	}

	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Kind != "delete" || snaps[0].Table != "users" {
		t.Fatalf("unexpected snapshot metadata: %+v", snaps[0])
	}
	if got := countRows(t, uc, "SELECT name FROM users WHERE id = 1"); strings.Contains(got, "alice") {
		t.Fatalf("row should be gone before rollback:\n%s", got)
	}

	restore, err := uc.RollbackSnapshot(ctx, "db1", snaps[0].ID)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(restore), "restored") {
		t.Fatalf("unexpected rollback output:\n%s", restore)
	}
	if got := countRows(t, uc, "SELECT name FROM users WHERE id = 1"); !strings.Contains(got, "alice") {
		t.Fatalf("row not restored:\n%s", got)
	}
}

// TestUpdateSnapshotRollback: an UPDATE is reversible when the table has an
// id column to target the restore with.
func TestUpdateSnapshotRollback(t *testing.T) {
	uc, ctx := seedUsers(t)

	if _, err := uc.ExecuteStatement(ctx, "db1", "UPDATE users SET name = 'hacked' WHERE id = 2", nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 || snaps[0].Kind != "update" {
		t.Fatalf("expected an update snapshot, got %+v", snaps)
	}

	if _, err := uc.RollbackSnapshot(ctx, "db1", snaps[0].ID); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	got := countRows(t, uc, "SELECT name FROM users WHERE id = 2")
	if strings.Contains(got, "hacked") || !strings.Contains(got, "bob") {
		t.Fatalf("update not reversed:\n%s", got)
	}
}

// TestSnapshotOnlyForMutations: reads and INSERTs consume no snapshot slots
// (there is nothing reversible to capture).
func TestSnapshotOnlyForMutations(t *testing.T) {
	uc, ctx := seedUsers(t)

	if _, err := uc.ExecuteQuery(ctx, "db1", "SELECT * FROM users", nil); err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if _, err := uc.ExecuteStatement(ctx, "db1", "INSERT INTO users (id, name) VALUES (9, 'z')", nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if got := len(uc.ListSnapshots("db1")); got != 0 {
		t.Fatalf("reads and INSERTs must not snapshot; got %d", got)
	}
}

// TestSnapshotMissingWhereCapturesEverything: an unbounded DELETE must
// snapshot the whole table, because that is what it will remove.
func TestSnapshotMissingWhereCapturesEverything(t *testing.T) {
	uc, ctx := seedUsers(t)

	if _, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM users", nil); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if len(snaps[0].Rows) != 2 {
		t.Fatalf("expected both rows captured, got %d", len(snaps[0].Rows))
	}

	if _, err := uc.RollbackSnapshot(ctx, "db1", snaps[0].ID); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if got := countRows(t, uc, "SELECT name FROM users"); !strings.Contains(got, "alice") || !strings.Contains(got, "bob") {
		t.Fatalf("both rows should be back:\n%s", got)
	}
}

// TestSnapshotSubqueryWhereIsNotTheFilter: a WHERE inside a subquery is not
// the statement's filter, so capturing on it would snapshot the wrong rows.
func TestSnapshotSubqueryWhereIsNotTheFilter(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE keep (id INTEGER PRIMARY KEY)`,
		`INSERT INTO users (id, name) VALUES (1, 'alice'), (2, 'bob')`,
		`INSERT INTO keep (id) VALUES (1)`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	// Deletes every user (no top-level WHERE); the subquery's WHERE must not
	// narrow the capture to just id = 1.
	if _, err := uc.ExecuteStatement(context.Background(), "db1",
		"DELETE FROM users WHERE id IN (SELECT id FROM keep WHERE id = 1)", nil); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if !strings.Contains(strings.ToUpper(snaps[0].Where), "WHERE") {
		t.Fatalf("a top-level WHERE exists and should be recorded: %q", snaps[0].Where)
	}
	if len(snaps[0].Rows) != 1 {
		t.Fatalf("the top-level filter selects one row; captured %d", len(snaps[0].Rows))
	}
}

// TestRollbackUnknownSnapshotErrors locks in clear failure semantics.
func TestRollbackUnknownSnapshotErrors(t *testing.T) {
	uc, ctx := seedUsers(t)
	if _, err := uc.RollbackSnapshot(ctx, "db1", "snap_999"); err == nil {
		t.Fatal("an unknown snapshot id must error")
	}
}

// TestSnapshotRingCap proves bounded retention per database.
func TestSnapshotRingCap(t *testing.T) {
	uc, ctx := seedUsers(t)
	for i := 0; i < snapshotCapacityPerDB+10; i++ {
		if _, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM users WHERE id = 1", nil); err != nil {
			t.Fatalf("delete %d failed: %v", i, err)
		}
		// Re-add so the next delete keeps capturing a row.
		if _, err := uc.ExecuteStatement(ctx, "db1", "INSERT OR IGNORE INTO users (id, name) VALUES (1, 'alice')", nil); err != nil {
			t.Fatalf("reseed failed: %v", err)
		}
	}
	if got := len(uc.ListSnapshots("db1")); got != snapshotCapacityPerDB {
		t.Fatalf("ring cap must bound retention at %d; got %d", snapshotCapacityPerDB, got)
	}
}

// TestSnapshotCarriesOriginatingStatement: a snapshot without provenance is a
// weak audit artifact, so it records the statement that caused it.
func TestSnapshotCarriesOriginatingStatement(t *testing.T) {
	uc, ctx := seedUsers(t)

	if _, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM users WHERE id = 1", nil); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	snaps := uc.ListSnapshots("db1")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if !strings.Contains(snaps[0].Statement, "DELETE FROM users") {
		t.Fatalf("snapshot missing its originating statement: %q", snaps[0].Statement)
	}
}

// TestSnapshotFailureDoesNotBlockExecution: the safety net must never become a
// new failure mode for the mutation the operator asked for.
func TestSnapshotFailureDoesNotBlockExecution(t *testing.T) {
	uc, ctx := seedUsers(t)

	// A table that does not exist: the capture query fails, the statement
	// still runs (and fails on its own terms, not on the snapshot's).
	_, err := uc.ExecuteStatement(ctx, "db1", "DELETE FROM does_not_exist", nil)
	if err == nil {
		t.Fatal("expected the DELETE itself to fail")
	}
	if strings.Contains(err.Error(), "snapshot") {
		t.Fatalf("the error must come from the statement, not the snapshot: %v", err)
	}
	if got := len(uc.ListSnapshots("db1")); got != 0 {
		t.Fatalf("a failed capture must not register a snapshot; got %d", got)
	}
}
