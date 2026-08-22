package usecase

import (
	"context"
	"testing"
)

// TestScanContentPII_FindsHiddenPII proves columns with innocent names but
// PII-bearing content get flagged.
func TestScanContentPII_FindsHiddenPII(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE events (id INTEGER PRIMARY KEY, notes TEXT, label TEXT)`)
	must(`INSERT INTO events (notes, label) VALUES ('contact jane@corp.io', 'sync')`)
	must(`INSERT INTO events (notes, label) VALUES ('called +1-555-234-5678', 'ops')`)
	must(`INSERT INTO events (notes, label) VALUES ('plain note', 'misc')`)

	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	findings, err := uc.ScanContentPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	got := map[string]bool{}
	for _, f := range findings {
		for _, cat := range f.Categories {
			got[f.Table+"."+f.Column+"|"+cat] = true
		}
	}
	if !got["events.notes|email"] {
		t.Fatalf("email-in-notes not detected: %+v", findings)
	}
	if !got["events.notes|phone"] {
		t.Fatalf("phone-in-notes not detected: %+v", findings)
	}
	for _, f := range findings {
		if f.Column == "label" || f.Column == "id" {
			t.Fatalf("benign column flagged: %+v", f)
		}
	}
}

// TestScanContentPII_SkipsNameFlaggedColumns proves name-heuristic columns
// are not double-reported by the content scan.
func TestScanContentPII_SkipsNameFlaggedColumns(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, email TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO people (email) VALUES ('a@b.io')`); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	findings, err := uc.ScanContentPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	for _, f := range findings {
		if f.Column == "email" {
			t.Fatal("name-flagged column must be excluded from content scan")
		}
	}
}

// TestScanContentPII_RespectsSampleCap proves the sampler reads at most N rows.
func TestScanContentPII_RespectsSampleCap(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, payload TEXT)`); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := raw.Exec(`INSERT INTO t (payload) VALUES (?)`, "reach me at x@y.io"); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})

	findings, err := uc.ScanContentPII(context.Background(), "db1", 3)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	for _, f := range findings {
		if f.Column == "payload" && f.SamplesScanned > 3 {
			t.Fatalf("sample cap violated: scanned %d", f.SamplesScanned)
		}
	}
	if len(findings) == 0 {
		t.Fatal("expected payload to be flagged")
	}
}
