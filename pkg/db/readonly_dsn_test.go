package db

import (
	"strings"
	"testing"

	"github.com/lib/pq"
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

	// Operator-supplied options are kept, and the read-only default is added to them.
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
	if !strings.Contains(custom, "-c default_transaction_read_only=on") {
		t.Fatalf("read-only default must be kept alongside custom options, got %q", custom)
	}
}

// TestBuildPostgresConnStr_ReadOnlyKeepsOperatorOptions proves that a read-only
// connection whose operator also sets `options` gets the operator's flags and
// the read-only session default in one value that the driver parses back.
func TestBuildPostgresConnStr_ReadOnlyKeepsOperatorOptions(t *testing.T) {
	operator := map[string]string{"options": "-c search_path=public"}
	dsn := buildPostgresConnStr(Config{
		Type: "postgres", Host: "localhost", Port: 5432, User: "u", Name: "db1",
		SSLMode: "disable", ReadOnly: true, Options: operator,
	})

	conn, err := pq.NewConnector(dsn)
	if err != nil {
		t.Fatalf("pq.NewConnector(%q) failed: %v", dsn, err)
	}
	want := "-c search_path=public -c default_transaction_read_only=on"
	if got := pqConnectorOpt(conn, "options"); got != want {
		t.Fatalf("parsed options = %q, want %q (dsn=%q)", got, want, dsn)
	}
	if n := strings.Count(dsn, "options="); n != 1 {
		t.Fatalf("options must be emitted exactly once, got %d in %q", n, dsn)
	}
	if len(operator) != 1 || operator["options"] != "-c search_path=public" {
		t.Fatalf("caller's Options map was mutated: %v", operator)
	}
}

// TestBuildPostgresConnStr_ReadOnlyWithoutOptionsUnchanged pins the read-only
// DSN for a connection without operator options to the output it always had.
func TestBuildPostgresConnStr_ReadOnlyWithoutOptionsUnchanged(t *testing.T) {
	dsn := buildPostgresConnStr(Config{
		Type: "postgres", Host: "localhost", Port: 5432, User: "u", Name: "db1",
		SSLMode: "disable", ReadOnly: true,
	})
	want := "host=localhost port=5432 user=u dbname=db1 sslmode=disable options='-c default_transaction_read_only=on'"
	if dsn != want {
		t.Fatalf("read-only DSN changed:\n got %q\nwant %q", dsn, want)
	}
}

// TestBuildPostgresConnStr_WritableWithOptionsUnchanged pins the DSN for a
// writable connection with operator options to the output it always had.
func TestBuildPostgresConnStr_WritableWithOptionsUnchanged(t *testing.T) {
	dsn := buildPostgresConnStr(Config{
		Type: "postgres", Host: "localhost", Port: 5432, User: "u", Name: "db1",
		SSLMode: "disable", Options: map[string]string{"options": "-c search_path=public"},
	})
	want := "host=localhost port=5432 user=u dbname=db1 sslmode=disable options=-c+search_path%3Dpublic"
	if dsn != want {
		t.Fatalf("writable DSN changed:\n got %q\nwant %q", dsn, want)
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
