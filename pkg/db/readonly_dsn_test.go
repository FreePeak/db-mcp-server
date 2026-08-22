package db

import (
	"strings"
	"testing"
)

// TestBuildPostgresConnStr_ReadOnlyEngineEnforcement locks in the
// engine-level read-only session default injected into the PostgreSQL DSN.
func TestBuildPostgresConnStr_ReadOnlyEngineEnforcement(t *testing.T) {
	dsn := buildPostgresConnStr(Config{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "u",
		Name:     "db1",
		ReadOnly: true,
	})
	if !strings.Contains(dsn, "default_transaction_read_only=on") {
		t.Fatalf("expected read-only session default in DSN, got %q", dsn)
	}

	writable := buildPostgresConnStr(Config{
		Type: "postgres", Host: "localhost", Port: 5432, User: "u", Name: "db1",
	})
	if strings.Contains(writable, "default_transaction_read_only") {
		t.Fatalf("writable database must not get read-only default, got %q", writable)
	}

	// Operator-supplied options win over automatic injection.
	custom := buildPostgresConnStr(Config{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "u",
		Name:     "db1",
		ReadOnly: true,
		Options:  map[string]string{"options": "-c application_name=custom"},
	})
	if strings.Count(custom, "options=") != 1 || !strings.Contains(custom, "application_name") {
		t.Fatalf("custom options must not be clobbered, got %q", custom)
	}
	if strings.Contains(custom, "default_transaction_read_only") {
		t.Fatalf("automatic injection must be skipped when custom options are set, got %q", custom)
	}
}

// TestMySQLDSN_ReadOnlyEngineEnforcement locks in the transaction_read_only
// system variable injected into the MySQL DSN for pooled connections.
func TestMySQLDSN_ReadOnlyEngineEnforcement(t *testing.T) {
	dsn := buildMySQLConnStr(Config{
		Type: "mysql", Host: "localhost", Port: 3306,
		User: "u", Password: "p", Name: "db1", ReadOnly: true,
	})
	if !strings.Contains(dsn, "transaction_read_only=1") {
		t.Fatalf("expected transaction_read_only=1 in DSN, got %q", dsn)
	}
	if !strings.Contains(dsn, "parseTime=true") {
		t.Fatalf("existing params must be preserved, got %q", dsn)
	}

	writable := buildMySQLConnStr(Config{
		Type: "mysql", Host: "localhost", Port: 3306,
		User: "u", Password: "p", Name: "db1",
	})
	if strings.Contains(writable, "transaction_read_only") {
		t.Fatalf("writable database must not get read-only default, got %q", writable)
	}
}
