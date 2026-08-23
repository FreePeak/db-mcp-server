package usecase

import (
	"context"
	"fmt"
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

// TestContentThresholdMet covers cycle 57: a category must match at least
// 5% of scanned samples (minimum one) before the column is reported. One
// stray match in a large sample is noise, not PII.
func TestContentThresholdMet(t *testing.T) {
	tests := []struct {
		hits, scanned int
		want          bool
	}{
		{1, 3, true},    // tiny table: single hit is strong signal
		{1, 20, true},   // exactly at the 5% line
		{1, 21, false},  // below the line: noise
		{5, 100, true},  // exactly 5%
		{4, 100, false}, // just under
		{50, 1000, true},
	}
	for _, tt := range tests {
		if got := contentThresholdMet(tt.hits, tt.scanned); got != tt.want {
			t.Fatalf("contentThresholdMet(%d, %d) = %v, want %v", tt.hits, tt.scanned, got, tt.want)
		}
	}
}

// TestScanContentPII_SuppressesNoise proves a lone phone-shaped order id in
// a large sample no longer flags the column, while a genuinely dense email
// column still does.
func TestScanContentPII_SuppressesNoise(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE t (id INTEGER PRIMARY KEY, payload TEXT, contacts TEXT)`)
	for i := 0; i < 100; i++ {
		payload := fmt.Sprintf("plain note %d", i)
		if i == 0 {
			payload = "order 0987654321" // one phone-shaped false positive
		}
		contacts := "no pii here"
		if i < 10 {
			contacts = "reach me at user" + fmt.Sprint(i) + "@corp.io"
		}
		must(fmt.Sprintf(`INSERT INTO t (payload, contacts) VALUES ('%s', '%s')`, payload, contacts))
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	findings, err := uc.ScanContentPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	flagged := map[string][]string{}
	for _, f := range findings {
		flagged[f.Column] = f.Categories
	}
	if _, noisy := flagged["payload"]; noisy {
		t.Fatalf("noise column flagged despite 1%% hit rate: %+v", findings)
	}
	if _, ok := flagged["contacts"]; !ok {
		t.Fatalf("dense email column not flagged: %+v", findings)
	}
}

// TestScanContentPII_ReportsHitCounts proves cycle 58: findings carry the
// per-category hit counts so operators can tune the noise floor themselves.
func TestScanContentPII_ReportsHitCounts(t *testing.T) {
	raw := openSQLiteForTest(t)
	must := func(q string) {
		t.Helper()
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("exec failed: %v", err)
		}
	}
	must(`CREATE TABLE t (id INTEGER PRIMARY KEY, notes TEXT)`)
	for i := 0; i < 10; i++ {
		must(fmt.Sprintf(`INSERT INTO t (notes) VALUES ('mail me at u%d@corp.io')`, i))
	}
	wrapper := &sqliteDB{db: raw}
	uc := NewDatabaseUseCase(&fakeRepo{db: wrapper, dbType: "sqlite"})
	findings, err := uc.ScanContentPII(context.Background(), "db1", 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	var notes *ContentPIIFinding
	for i := range findings {
		if findings[i].Column == "notes" {
			notes = &findings[i]
		}
	}
	if notes == nil || notes.Hits["email"] != 10 {
		t.Fatalf("expected notes with email hits=10, got %+v", notes)
	}
}
