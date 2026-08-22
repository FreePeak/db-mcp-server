package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestLiveEngineStats queries engine health indicators through the same
// catalog paths the health tool uses. Requires the docker-compose.test.yml
// stack; skips when it is not running.
func TestLiveEngineStats(t *testing.T) {
	cases := []struct {
		name  string
		cfg   Config
		query string
	}{
		{
			name:  "postgres buffer cache hit ratio",
			cfg:   Config{Type: "postgres", Host: "localhost", Port: 15432, User: "user1", Password: "password1", Name: "db1"},
			query: `SELECT round(100.0 * blks_hit / NULLIF(blks_hit + blks_read, 0)) AS ratio FROM pg_stat_database WHERE datname = current_database()`,
		},
		{
			name:  "mysql innodb buffer efficiency",
			cfg:   Config{Type: "mysql", Host: "localhost", Port: 13306, User: "user1", Password: "password1", Name: "db1"},
			query: `SELECT round(100.0 * (1 - variable_value / NULLIF((SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'Innodb_buffer_pool_read_requests'), 0)), 2) AS ratio FROM performance_schema.global_status WHERE variable_name = 'Innodb_buffer_pool_reads'`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, err := NewDatabase(tc.cfg)
			if err != nil {
				t.Fatalf("failed to create database: %v", err)
			}
			if err := database.Connect(); err != nil {
				if isConnRefused(err) {
					t.Skipf("test container not reachable, skipping: %v", err)
				}
				t.Fatalf("failed to connect: %v", err)
			}
			defer func() { _ = database.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			rows, err := database.Query(ctx, tc.query)
			if err != nil {
				t.Fatalf("engine stats query failed: %v", err)
			}
			defer func() { _ = rows.Close() }()

			columns, err := rows.Columns()
			if err != nil || len(columns) == 0 {
				t.Fatalf("expected result columns, got %v (err %v)", columns, err)
			}

			values := make([]interface{}, len(columns))
			ptrs := make([]interface{}, len(columns))
			for i := range columns {
				ptrs[i] = &values[i]
			}
			if !rows.Next() {
				t.Fatalf("expected at least one row; rows.Err=%v", rows.Err())
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			var ratio string
			switch v := values[0].(type) {
			case []byte:
				ratio = string(v)
			case string:
				ratio = v
			case float64:
				ratio = fmt.Sprintf("%.2f", v)
			case int64:
				ratio = fmt.Sprintf("%d", v)
			default:
				t.Fatalf("unexpected ratio type %T", values[0])
			}
			if strings.TrimSpace(ratio) == "" {
				t.Fatal("expected a non-empty ratio value")
			}
		})
	}
}
