package usecase

import (
	"context"
	"strings"
	"testing"
)

// TestBloatCatalog proves the catalog SELECT reads dead/live tuple
// counts per user table from pg_stat_user_tables.
func TestBloatCatalog(t *testing.T) {
	q := bloatQuery("postgres")
	if !strings.Contains(q, "n_dead_tup") || !strings.Contains(q, "pg_stat_user_tables") {
		t.Fatalf("catalog wrong:\n%s", q)
	}
	if bloatQuery("mysql") != "" || bloatQuery("sqlite") != "" {
		t.Fatal("only postgres exposes pg_stat_user_tables")
	}
}

// TestCheckTableBloat_Unsupported proves unsupported engines get an
// explicit error.
func TestCheckTableBloat_Unsupported(t *testing.T) {
	raw := openSQLiteForTest(t)
	uc := NewDatabaseUseCase(&fakeRepo{db: &sqliteDB{db: raw}, dbType: "sqlite"})
	if _, err := uc.CheckTableBloat(context.Background(), "db1"); err == nil ||
		!strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

// TestBloatVerdict proves the dead-tuple ratio thresholds escalate and
// that tiny tables stay silent to keep the report actionable.
func TestBloatVerdict(t *testing.T) {
	tests := []struct {
		name       string
		live, dead int64
		wantSub    string // "" = healthy tables must render empty
	}{
		{"clean", 10_000, 100, ""},
		{"tiny_table_skipped", 500, 400, ""},
		{"warning_zone", 10_000, 2_500, "WARNING"},
		{"critical_zone", 10_000, 11_000, "CRITICAL"},
		{"boundary_warning", 8_000, 2_000, "WARNING"}, // exactly 20%
		{"boundary_critical", 10_000, 10_000, "CRITICAL"},
		{"all_dead", 0, 5_000, "CRITICAL"},
		{"zero_rows", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bloatVerdict("events", tt.live, tt.dead)
			switch tt.wantSub {
			case "":
				if got != "" {
					t.Fatalf("healthy/tiny table rendered a line:\n%s", got)
				}
			default:
				if !strings.Contains(got, tt.wantSub) {
					t.Fatalf("missing %q in:\n%s", tt.wantSub, got)
				}
				if !strings.Contains(got, "events") {
					t.Fatalf("table name missing:\n%s", got)
				}
			}
		})
	}
}

// TestRenderBloatReport proves worst-first ordering, clean-report
// wording, and the autovacuum hint.
func TestRenderBloatReport(t *testing.T) {
	rows := []bloatRow{
		{"users", 8_000, 3_000},  // ~27% WARNING
		{"orders", 1_000, 9_000}, // ~90% CRITICAL
		{"events", 50_000, 100},  // healthy
		{"tiny", 500, 400},       // skipped
	}
	out := renderBloatReport(rows)
	if !strings.Contains(out, "2 of 4") {
		t.Fatalf("expected 2-of-4 header, got:\n%s", out)
	}
	iOrders := strings.Index(out, "orders")
	iUsers := strings.Index(out, "users")
	if iOrders < 0 || iUsers < 0 || iOrders > iUsers {
		t.Fatalf("worst table not first:\n%s", out)
	}
	if !strings.Contains(out, "VACUUM") || !strings.Contains(out, "long-running transaction") {
		t.Fatalf("remediation hint missing:\n%s", out)
	}

	clean := renderBloatReport([]bloatRow{{"events", 50_000, 100}})
	if !strings.Contains(clean, "No significant bloat") || strings.Contains(clean, "WARNING") {
		t.Fatalf("clean report wrong:\n%s", clean)
	}
}
