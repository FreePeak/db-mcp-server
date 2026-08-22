package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestReadOnlyEngineEnforcement_Live verifies that read_only databases reject
// writes at the DATABASE ENGINE level, not just via application-layer
// checks. Requires the docker-compose.test.yml stack (MySQL on 13306,
// PostgreSQL on 15432); skips when the stack is not running.
func TestReadOnlyEngineEnforcement_Live(t *testing.T) {
	cases := []struct {
		name   string
		config Config
	}{
		{
			name: "postgres",
			config: Config{
				Type:     "postgres",
				Host:     "localhost",
				Port:     15432,
				User:     "user1",
				Password: "password1",
				Name:     "db1",
			},
		},
		{
			name: "mysql",
			config: Config{
				Type:     "mysql",
				Host:     "localhost",
				Port:     13306,
				User:     "user1",
				Password: "password1",
				Name:     "db1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.config
			cfg.ReadOnly = true

			database, err := NewDatabase(cfg)
			if err != nil {
				t.Fatalf("failed to create database: %v", err)
			}
			if err := database.Connect(); err != nil {
				if isConnRefused(err) {
					t.Skipf("%s test container not reachable, skipping: %v", tc.name, err)
				}
				t.Fatalf("failed to connect: %v", err)
			}
			defer func() { _ = database.Close() }()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Reads must work on a read-only connection.
			rows, err := database.Query(ctx, "SELECT 1")
			if err != nil {
				t.Fatalf("SELECT failed on read-only %s connection: %v", tc.name, err)
			}
			if err := rows.Close(); err != nil {
				t.Fatalf("failed to close rows: %v", err)
			}

			// Writes must be rejected by the engine itself.
			if _, err := database.Exec(ctx, "CREATE TABLE IF NOT EXISTS _ro_probe (id INT)"); err == nil {
				t.Fatal("expected CREATE TABLE to be rejected by the engine on a read-only connection")
			} else if !isReadOnlyRejection(err) {
				t.Fatalf("expected a read-only rejection from the engine, got: %v", err)
			}
		})
	}
}

func isConnRefused(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connect: no such file") ||
		strings.Contains(err.Error(), "i/o timeout"))
}

func isReadOnlyRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "read-only") ||
		strings.Contains(msg, "read only") ||
		strings.Contains(msg, "readonly")
}
