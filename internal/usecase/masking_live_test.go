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
