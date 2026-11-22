package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// TestExecuteQuery_MaskingLive proves the name-based masking pipeline
// against real engine drivers, where cells arrive as engine-specific
// types ([]byte from MySQL, typed values elsewhere) rather than the
// interface{} shapes SQLite fakes produce. Skips when the throwaway
// engines from scripts/live-db-setup.sh are unreachable.
func TestExecuteQuery_MaskingLive(t *testing.T) {
	rules := []db.MaskingRule{
		{Pattern: "(?i)^email$", Strategy: "fixed_string", Value: "***MASKED***"},
		{Pattern: "(?i)^phone$", Strategy: "partial", KeepLast: 4},
	}

	cases := []struct {
		name   string
		driver string
		dsn    string
		dbType string
		dbID   string
	}{
		{"postgres", "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable", "postgres", "pg_live"},
		{"mysql", "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true", "mysql", "my_live"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := openLive(t, tc.driver, tc.dsn)
			// Seed idempotently; ignore errors when objects exist.
			switch tc.driver {
			case "postgres":
				_, _ = g.db.Exec(`DROP TABLE IF EXISTS mask_users`)
				_, _ = g.db.Exec(`CREATE TABLE mask_users (id SERIAL PRIMARY KEY, email TEXT, phone VARCHAR(20))`)
				_, _ = g.db.Exec(`INSERT INTO mask_users (email, phone) VALUES ('alice@example.com', '5551234567')`)
			case "mysql":
				_, _ = g.db.Exec(`CREATE TABLE IF NOT EXISTS db1.mask_users (id INT PRIMARY KEY AUTO_INCREMENT, email VARCHAR(200), phone VARCHAR(20))`)
				_, _ = g.db.Exec(`DELETE FROM db1.mask_users`)
				_, _ = g.db.Exec(`INSERT INTO db1.mask_users (email, phone) VALUES ('alice@example.com', '5551234567')`)
			}
			mdb := &maskedDB{Database: g, rules: rules}
			uc := NewDatabaseUseCase(&fakeRepo{db: mdb, dbType: tc.dbType})

			out, err := uc.ExecuteQuery(context.Background(), tc.dbID, "SELECT id, email, phone FROM mask_users", nil)
			if err != nil {
				t.Fatalf("masked query failed: %v", err)
			}

			if !strings.Contains(out, "***MASKED***") {
				t.Errorf("expected fixed_string mask through %s driver, got:\n%s", tc.driver, out)
			}
			// partial keeps only the last four digits visible.
			if !strings.Contains(out, "******4567") && !strings.Contains(out, "*****4567") {
				t.Errorf("expected partially masked phone keeping last 4 digits, got:\n%s", out)
			}
			if strings.Contains(out, "alice@example.com") || strings.Contains(out, "5551234567") {
				t.Errorf("raw sensitive values leaked through masking on %s:\n%s", tc.driver, out)
			}
			if !strings.Contains(out, "Masked cells: 2") {
				t.Errorf("expected masked-cell footer, got:\n%s", out)
			}
		})
	}
}

// TestExecuteQueryMaskedPath_Live drives the entry the query tool uses
// (ExecuteQueryMasked with mask_pii absent, so mask=false) against each live
// engine and asserts the operator's name-based rule masks the column. Skips
// when the compose stack is unreachable (see openLive).
func TestExecuteQueryMaskedPath_Live(t *testing.T) {
	rules := []db.MaskingRule{{Pattern: "(?i)^email$", Strategy: "fixed_string", Value: "***MASKED***"}}

	cases := []struct {
		name   string
		driver string
		dsn    string
		dbID   string
	}{
		{"postgres", "postgres", "host=localhost port=15432 user=user1 password=password1 dbname=db1 sslmode=disable", "pg_query_path"},
		{"mysql", "mysql", "user1:password1@tcp(localhost:13306)/db1?parseTime=true", "my_query_path"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := openLive(t, tc.driver, tc.dsn)
			for _, s := range []string{
				`DROP TABLE IF EXISTS mask_path_users`,
				`CREATE TABLE mask_path_users (id INTEGER PRIMARY KEY, email VARCHAR(200))`,
				`INSERT INTO mask_path_users (id, email) VALUES (1, 'alice@example.com')`,
			} {
				if _, err := g.db.Exec(s); err != nil {
					t.Fatalf("seed failed on %s: %v", tc.driver, err)
				}
			}
			uc := NewDatabaseUseCase(&fakeRepo{db: &maskedDB{Database: g, rules: rules}, dbType: tc.driver})

			out, err := uc.ExecuteQueryMasked(context.Background(), tc.dbID, "SELECT id, email FROM mask_path_users", nil, false)
			if err != nil {
				t.Fatalf("query path failed on %s: %v", tc.driver, err)
			}
			if !strings.Contains(out, "***MASKED***") {
				t.Errorf("expected fixed_string mask through query path on %s, got:\n%s", tc.driver, out)
			}
			if strings.Contains(out, "alice@example.com") {
				t.Errorf("raw email leaked through query path on %s:\n%s", tc.driver, out)
			}
			if !strings.Contains(out, "Masked cells: 1") {
				t.Errorf("expected masked-cell footer on %s, got:\n%s", tc.driver, out)
			}
		})
	}
}
