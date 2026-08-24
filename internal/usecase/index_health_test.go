package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestIndexHealth_FindsDuplicatesAndRedundancy seeds a table with an exact
// duplicate pair and a redundant prefix pair, then checks both findings and
// the engine-appropriate DROP syntax.
func TestIndexHealth_FindsDuplicatesAndRedundancy(t *testing.T) {
	raw := openSQLiteForTest(t)
	ddl := []string{
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER, region TEXT, total REAL)`,
		`CREATE INDEX idx_orders_customer ON orders (customer_id)`,
		`CREATE INDEX idx_orders_customer_copy ON orders (customer_id)`,       // exact duplicate
		`CREATE INDEX idx_orders_cust_region ON orders (customer_id, region)`, // prefix of above
	}
	for _, s := range ddl {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.IndexHealth(context.Background(), "db")
	if err != nil {
		t.Fatalf("index_health failed: %v", err)
	}

	if !strings.Contains(out, "DUPLICATE on orders") {
		t.Errorf("expected duplicate finding, got:\n%s", out)
	}
	if !strings.Contains(out, "REDUNDANT on orders") {
		t.Errorf("expected redundant finding, got:\n%s", out)
	}
	// Canonicalization: the DUPLICATE finding keeps the alphabetically
	// smaller name and drops the copy.
	if !strings.Contains(out, "idx_orders_customer_copy duplicates idx_orders_customer") {
		t.Errorf("expected canonical keeper to be the smaller name, got:\n%s", out)
	}
	// Prefix redundancy then runs over the canonical set only, so the
	// verdict is consistent: keep cust_region, drop the plain customer index.
	if !strings.Contains(out, "DROP INDEX idx_orders_customer;") {
		t.Errorf("expected DROP for the prefix-covered index, got:\n%s", out)
	}
}

// TestIndexHealth_CleanDatabase verifies the no-findings path end to end.
func TestIndexHealth_CleanDatabase(t *testing.T) {
	raw := openSQLiteForTest(t)
	for _, s := range []string{
		`CREATE TABLE items (id INTEGER PRIMARY KEY, sku TEXT)`,
		`CREATE UNIQUE INDEX idx_items_sku ON items (sku)`,
	} {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.IndexHealth(context.Background(), "db")
	if err != nil {
		t.Fatalf("index_health failed: %v", err)
	}
	if !strings.Contains(out, "No duplicate or redundant indexes") {
		t.Errorf("expected clean report, got:\n%s", out)
	}
}

// TestParseIndexRows_MySQLShape locks in the grouped SHOW INDEX parsing:
// multi-row indexes must assemble columns by Seq_in_index order.
func TestParseIndexRows_MySQLShape(t *testing.T) {
	rows := []map[string]interface{}{
		{"Key_name": "idx_ab", "Seq_in_index": int64(1), "Column_name": "a", "Non_unique": int64(1)},
		{"Key_name": "idx_b", "Seq_in_index": int64(1), "Column_name": "b", "Non_unique": int64(1)},
		{"Key_name": "idx_ab", "Seq_in_index": int64(2), "Column_name": "c", "Non_unique": int64(1)},
	}
	indexes := parseIndexRows(rows)
	if len(indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(indexes))
	}
	byName := map[string]parsedIndex{}
	for _, ix := range indexes {
		byName[ix.name] = ix
	}
	if got := strings.Join(byName["idx_ab"].cols, ","); got != "a,c" {
		t.Errorf("expected idx_ab cols a,c, got %q", got)
	}
	if got := strings.Join(byName["idx_b"].cols, ","); got != "b" {
		t.Errorf("expected idx_b col b, got %q", got)
	}
}

// TestUsageFindings_Formatters locks in cycle 27's statistics-driven
// findings for both engines' row shapes.
func TestUsageFindings_Formatters(t *testing.T) {
	pg := formatUnusedFindings([]map[string]interface{}{
		{"table_name": "orders", "index_name": "idx_orders_region"},
	})
	if len(pg) != 1 || !strings.Contains(pg[0], "UNUSED on orders") || !strings.Contains(pg[0], "DROP INDEX idx_orders_region;") {
		t.Errorf("unexpected pg unused finding: %v", pg)
	}

	inv := formatInvalidFindings([]map[string]interface{}{
		{"index_name": "idx_users_email"},
	})
	if len(inv) != 1 || !strings.Contains(inv[0], "INVALID") || !strings.Contains(inv[0], "idx_users_email") {
		t.Errorf("unexpected invalid finding: %v", inv)
	}

	my := formatMySQLUnusedFindings([]map[string]interface{}{
		{"table_name": "orders", "index_name": "idx_orders_total"},
	})
	if len(my) != 1 || !strings.Contains(my[0], "ALTER TABLE `orders` DROP INDEX `idx_orders_total`;") {
		t.Errorf("unexpected mysql unused finding: %v", my)
	}
}

// TestIndexHealth_WithoutUsageStats verifies the graceful SQLite path: no
// statistics catalogs exist, yet the report still renders. The
// usage-unavailable footer only appears when there are findings; the clean
// database takes the no-findings early return.
func TestIndexHealth_WithoutUsageStats(t *testing.T) {
	raw := openSQLiteForTest(t)
	if _, err := raw.Exec(`CREATE TABLE t1 (id INTEGER PRIMARY KEY, a TEXT)`); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})

	out, err := uc.IndexHealth(context.Background(), "db")
	if err != nil {
		t.Fatalf("index_health failed: %v", err)
	}
	if !strings.Contains(out, "No duplicate or redundant indexes") {
		t.Errorf("expected clean report, got:\n%s", out)
	}

	// With a finding present, the footer must disclose that usage evidence
	// was unavailable rather than implying UNUSED claims are exhaustive.
	if _, err := raw.Exec(`CREATE INDEX idx_t1_a ON t1 (a); CREATE INDEX idx_t1_a_copy ON t1 (a)`); err != nil {
		t.Fatalf("seed indexes failed: %v", err)
	}
	out, err = uc.IndexHealth(context.Background(), "db")
	if err != nil {
		t.Fatalf("index_health failed: %v", err)
	}
	if !strings.Contains(out, "DUPLICATE on t1") || !strings.Contains(out, "ran without them") {
		t.Errorf("expected finding plus usage-unavailable footer, got:\n%s", out)
	}
}

// TestRedundancyFindings_UniquenessGuard verifies a non-unique smaller
// index under a unique larger one is flagged, but a unique smaller index
// under a non-unique larger one is not (uniqueness cannot be recovered).
func TestRedundancyFindings_UniquenessGuard(t *testing.T) {
	findings := redundancyFindings("sqlite", "t", []parsedIndex{
		{name: "u_small", cols: []string{"a"}, unique: true},
		{name: "nu_big", cols: []string{"a", "b"}, unique: false},
	})
	if len(findings) != 0 {
		t.Errorf("unique-under-non-unique must not be redundant, got %v", findings)
	}

	findings = redundancyFindings("sqlite", "t", []parsedIndex{
		{name: "nu_small", cols: []string{"a"}, unique: false},
		{name: "u_big", cols: []string{"a", "b"}, unique: true},
	})
	if len(findings) != 1 || !strings.Contains(findings[0], "REDUNDANT") {
		t.Errorf("non-unique under unique should be flagged, got %v", findings)
	}
}
