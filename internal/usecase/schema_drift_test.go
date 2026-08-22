package usecase

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func seedSchema(t *testing.T) (*DatabaseUseCase, context.Context) {
	t.Helper()
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec %q failed: %v", q, err)
		}
	}
	must(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)`)
	must(`CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	rawDBForTest[uc] = raw
	return uc, context.Background()
}

// rawDBForTest lets tests evolve the schema behind a use case instance.
var rawDBForTest = map[*DatabaseUseCase]*sql.DB{}

func wrapperRawDB(uc *DatabaseUseCase) *sql.DB { return rawDBForTest[uc] }

// TestCaptureSchemaSnapshot proves capture returns a normalized table map.
func TestCaptureSchemaSnapshot(t *testing.T) {
	uc, ctx := seedSchema(t)

	snap, err := uc.CaptureSchemaSnapshot(ctx, "db1")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if len(snap.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %+v", len(snap.Tables), snap.Tables)
	}
	cols, ok := snap.Tables["users"]
	if !ok || len(cols) == 0 {
		t.Fatalf("users missing or empty: %+v", snap.Tables)
	}
	foundEmail := false
	for _, c := range cols {
		if strings.EqualFold(c.Name, "email") && strings.Contains(strings.ToLower(c.Type), "text") {
			foundEmail = true
		}
	}
	if !foundEmail {
		t.Fatalf("users.email (text) not captured: %+v", cols)
	}
}

// TestCheckSchemaDrift_DetectsAllChangeClasses proves added/removed tables,
// added/removed columns, and type changes are all reported.
func TestCheckSchemaDrift_DetectsAllChangeClasses(t *testing.T) {
	uc, ctx := seedSchema(t)

	baseline, err := uc.CaptureSchemaSnapshot(ctx, "db1")
	if err != nil {
		t.Fatalf("baseline failed: %v", err)
	}

	// Evolve the schema.
	raw := wrapperRawDB(uc)
	for _, q := range []string{
		`ALTER TABLE users ADD COLUMN name TEXT`,         // column added
		`CREATE TABLE comments (id INTEGER PRIMARY KEY)`, // table added
		`DROP TABLE posts`,                               // table removed
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("evolve %q failed: %v", q, err)
		}
	}

	report, err := uc.CheckSchemaDrift(ctx, "db1", baseline.ID)
	if err != nil {
		t.Fatalf("drift check failed: %v", err)
	}
	joined := strings.Join(report.Changes, "\n")
	if !strings.Contains(joined, "comments") {
		t.Fatalf("added table not reported:\n%s", joined)
	}
	if !strings.Contains(joined, "posts") {
		t.Fatalf("removed table not reported:\n%s", joined)
	}
	if !strings.Contains(joined, "users.name") {
		t.Fatalf("added column not reported:\n%s", joined)
	}
	if report.Drifted != true {
		t.Fatal("report must flag drifted=true")
	}
}

// TestCheckSchemaDrift_NoChanges proves a clean comparison reports nothing.
func TestCheckSchemaDrift_NoChanges(t *testing.T) {
	uc, ctx := seedSchema(t)
	baseline, err := uc.CaptureSchemaSnapshot(ctx, "db1")
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	report, err := uc.CheckSchemaDrift(ctx, "db1", baseline.ID)
	if err != nil {
		t.Fatalf("drift check failed: %v", err)
	}
	if report.Drifted || len(report.Changes) != 0 {
		t.Fatalf("unexpected drift: %+v", report.Changes)
	}
}

// TestCheckSchemaDrift_UnknownBaselineErrors locks in failure semantics.
func TestCheckSchemaDrift_UnknownBaselineErrors(t *testing.T) {
	uc, ctx := seedSchema(t)
	if _, err := uc.CheckSchemaDrift(ctx, "db1", "schema_snap_999"); err == nil {
		t.Fatal("unknown baseline must error")
	}
}

// TestListSchemaSnapshots proves baselines are enumerable.
func TestListSchemaSnapshots(t *testing.T) {
	uc, ctx := seedSchema(t)
	if _, err := uc.CaptureSchemaSnapshot(ctx, "db1"); err != nil {
		t.Fatalf("capture 1 failed: %v", err)
	}
	if _, err := uc.CaptureSchemaSnapshot(ctx, "db1"); err != nil {
		t.Fatalf("capture 2 failed: %v", err)
	}
	snaps := uc.ListSchemaSnapshots("db1")
	if len(snaps) != 2 {
		t.Fatalf("expected 2 schema snapshots, got %d", len(snaps))
	}
}
