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
		{
			// Cycle 44: Oracle enforces read-only through privileges, not a
			// session flag — mcp_ro holds CREATE SESSION + SELECT grants only,
			// so the connector's privilege audit passes and the engine itself
			// refuses writes.
			name: "oracle",
			config: Config{
				Type:     "oracle",
				Host:     "localhost",
				Port:     1521,
				User:     "mcp_ro",
				Password: "mcp_ro_pass",
				Name:     "TESTDB",
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
			probe := "CREATE TABLE IF NOT EXISTS _ro_probe (id INT)"
			if tc.config.Type == "oracle" {
				// Oracle has no IF NOT EXISTS and forbids leading underscores.
				probe = "CREATE TABLE ro_probe (id INT)"
			}
			if _, err := database.Exec(ctx, probe); err == nil {
				t.Fatal("expected CREATE TABLE to be rejected by the engine on a read-only connection")
			} else if !isReadOnlyRejection(err) {
				t.Fatalf("expected a read-only rejection from the engine, got: %v", err)
			}
		})
	}
}

// TestReadOnlyOracleRefusesWritableCredentials locks in cycle 44's
// fail-closed audit: connecting with credentials that hold write
// privileges must abort at Connect time, never serve a database whose
// read_only flag cannot actually be enforced by the engine.
func TestReadOnlyOracleRefusesWritableCredentials(t *testing.T) {
	cfg := Config{
		Type:     "oracle",
		Host:     "localhost",
		Port:     1521,
		User:     "testuser", // schema owner: holds CREATE TABLE etc.
		Password: "testpass",
		Name:     "TESTDB",
		ReadOnly: true,
	}
	database, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	err = database.Connect()
	if err == nil {
		t.Fatal("expected Connect to refuse writable credentials for a read_only oracle database")
	} else if isConnRefused(err) {
		t.Skipf("oracle container not reachable, skipping: %v", err)
	}
	if !strings.Contains(err.Error(), "write privilege") && !strings.Contains(err.Error(), "fail closed") &&
		!strings.Contains(err.Error(), "read_only cannot hold") {
		t.Fatalf("expected the privilege-audit refusal, got: %v", err)
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
	if strings.Contains(msg, "read-only") ||
		strings.Contains(msg, "read only") ||
		strings.Contains(msg, "readonly") {
		return true
	}
	// Oracle has no session read-only flag: restricted credentials get
	// ORA-01031 insufficient privileges when the engine refuses the write.
	return strings.Contains(msg, "insufficient privileges") ||
		strings.Contains(msg, "ora-01031")
}
