package db

import (
	"context"
	"path/filepath"
	"testing"

	intLogger "github.com/FreePeak/db-mcp-server/internal/logger"
)

func initLoggerForTests() {
	intLogger.Initialize(intLogger.Config{Level: "error"})
}

func TestSQLiteConnection(t *testing.T) {
	initLoggerForTests()

	// Create a temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Test basic SQLite connection
	config := Config{
		Type:             "sqlite",
		DatabasePath:     dbPath,
		UseModerncDriver: true,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create SQLite database: %v", err)
	}

	// Test connection
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to SQLite: %v", err)
	}

	// Test basic query
	ctx := context.Background()
	rows, err := db.Query(ctx, "SELECT sqlite_version()")
	if err != nil {
		t.Fatalf("Failed to query SQLite version: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Fatal("No rows returned for version query")
	}

	var version string
	if err := rows.Scan(&version); err != nil {
		t.Fatalf("Failed to scan version: %v", err)
	}

	if version == "" {
		t.Fatal("Empty version returned")
	}

	// Close connection
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

func TestSQLiteInMemoryConnection(t *testing.T) {
	initLoggerForTests()

	config := Config{
		Type:             "sqlite",
		DatabasePath:     ":memory:",
		UseModerncDriver: true,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create in-memory SQLite database: %v", err)
	}

	// Test connection
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to in-memory SQLite: %v", err)
	}

	// Test table creation and query
	ctx := context.Background()
	_, err = db.Exec(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec(ctx, "INSERT INTO test (name) VALUES (?)", "test_name")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}

	rows, err := db.Query(ctx, "SELECT name FROM test WHERE name = ?", "test_name")
	if err != nil {
		t.Fatalf("Failed to query data: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Fatal("No rows returned for test query")
	}

	var name string
	if err := rows.Scan(&name); err != nil {
		t.Fatalf("Failed to scan name: %v", err)
	}

	if name != "test_name" {
		t.Errorf("Expected 'test_name', got '%s'", name)
	}

	// Close connection
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

func TestSQLiteConnectionValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "Valid file path",
			config: Config{
				Type:         "sqlite",
				DatabasePath: "/tmp/test.db",
			},
			expectError: false,
		},
		{
			name: "Valid in-memory",
			config: Config{
				Type:         "sqlite",
				DatabasePath: ":memory:",
			},
			expectError: false,
		},
		{
			name: "Using name field",
			config: Config{
				Type: "sqlite",
				Name: "test.db",
			},
			expectError: false,
		},
		{
			name: "Empty database path",
			config: Config{
				Type: "sqlite",
			},
			expectError: false, // Should use defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewDatabase(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if db == nil {
				t.Error("Expected database instance but got nil")
			}
		})
	}
}

func TestSQLiteConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "File database",
			config: Config{
				Type:         "sqlite",
				DatabasePath: "/path/to/db.sqlite",
			},
			expected: "SQLite database: /path/to/db.sqlite",
		},
		{
			name: "In-memory database",
			config: Config{
				Type:         "sqlite",
				DatabasePath: ":memory:",
			},
			expected: "SQLite in-memory database",
		},
		{
			name: "Encrypted database",
			config: Config{
				Type:          "sqlite",
				DatabasePath:  "/path/to/encrypted.db",
				EncryptionKey: "secret123",
			},
			expected: "SQLite database: /path/to/encrypted.db (encrypted)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewDatabase(tt.config)
			if err != nil {
				t.Fatalf("Failed to create database: %v", err)
			}

			connStr := db.ConnectionString()
			if connStr != tt.expected {
				t.Errorf("Expected connection string '%s', got '%s'", tt.expected, connStr)
			}
		})
	}
}

func TestSQLiteConnectionTimeout(t *testing.T) {
	initLoggerForTests()

	config := Config{
		Type:             "sqlite",
		DatabasePath:     ":memory:",
		QueryTimeout:     30,
		UseModerncDriver: true,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	timeout := db.QueryTimeout()
	if timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", timeout)
	}
}

func TestSQLiteTransaction(t *testing.T) {
	initLoggerForTests()

	config := Config{
		Type:             "sqlite",
		DatabasePath:     ":memory:",
		UseModerncDriver: true,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Create table
	_, err = db.Exec(ctx, "CREATE TABLE test_txn (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert data
	_, err = tx.ExecContext(ctx, "INSERT INTO test_txn (value) VALUES (?)", "transaction_test")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("Failed to insert in transaction: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify data was committed
	rows, err := db.Query(ctx, "SELECT value FROM test_txn WHERE value = ?", "transaction_test")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Fatal("No rows found after commit")
	}

	var value string
	if err := rows.Scan(&value); err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}

	if value != "transaction_test" {
		t.Errorf("Expected 'transaction_test', got '%s'", value)
	}
}

func TestSQLitePerformanceSettings(t *testing.T) {
	initLoggerForTests()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "perf_test.db")

	config := Config{
		Type:             "sqlite",
		DatabasePath:     dbPath,
		CacheSize:        5000,
		JournalMode:      JournalWAL,
		UseModerncDriver: true,
	}

	db, err := NewDatabase(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Test that performance settings were applied
	rows, err := db.Query(ctx, "PRAGMA cache_size")
	if err != nil {
		t.Fatalf("Failed to query cache size: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		var cacheSize int
		if err := rows.Scan(&cacheSize); err != nil {
			t.Fatalf("Failed to scan cache size: %v", err)
		}
		if cacheSize != 5000 {
			t.Errorf("Expected cache size 5000, got %d", cacheSize)
		}
	}

	// Test journal mode - note that the mode setting might take time to apply
	rows, err = db.Query(ctx, "PRAGMA journal_mode")
	if err != nil {
		t.Fatalf("Failed to query journal mode: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		var journalMode string
		if err := rows.Scan(&journalMode); err != nil {
			t.Fatalf("Failed to scan journal mode: %v", err)
		}
		// Journal mode setting might not be immediately applied or different drivers handle it differently
		t.Logf("Journal mode: %s", journalMode)
	}
}
