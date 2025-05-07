package timescale

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func TestNewTimescaleDB(t *testing.T) {
	// Create a basic config
	config := TimescaleDBConfig{
		PostgresConfig: db.Config{
			Type:     "postgres",
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpass",
			Name:     "testdb",
		},
		UseTimescaleDB: true,
	}

	// Create a new TimescaleDB instance directly with a mock database
	mockDB := NewMockDB()
	tsdb := &TimescaleDB{
		Database: mockDB,
		config:   config,
	}

	// Check that the config was stored
	if tsdb.config.UseTimescaleDB != config.UseTimescaleDB {
		t.Errorf("Expected UseTimescaleDB to be %v, got %v", config.UseTimescaleDB, tsdb.config.UseTimescaleDB)
	}
}

func TestConnect(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &TimescaleDB{
		Database:      mockDB,
		config:        TimescaleDBConfig{UseTimescaleDB: true},
		isTimescaleDB: false,
	}

	// Mock the QueryRow method to simulate a successful TimescaleDB detection
	mockDB.RegisterQueryResult("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'", "2.8.0", nil)

	// Connect to the database
	err := tsdb.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Check that the TimescaleDB extension was detected
	if !tsdb.isTimescaleDB {
		t.Error("Expected isTimescaleDB to be true, got false")
	}
	if tsdb.extVersion != "2.8.0" {
		t.Errorf("Expected extVersion to be '2.8.0', got '%s'", tsdb.extVersion)
	}

	// Test error case when database connection fails
	mockDB = NewMockDB()
	mockDB.SetConnectError(errors.New("mocked connection error"))
	tsdb = &TimescaleDB{
		Database:      mockDB,
		config:        TimescaleDBConfig{UseTimescaleDB: true},
		isTimescaleDB: false,
	}

	err = tsdb.Connect()
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	// Test case when TimescaleDB extension is not available
	mockDB = NewMockDB()
	mockDB.RegisterQueryResult("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'", nil, sql.ErrNoRows)
	tsdb = &TimescaleDB{
		Database:      mockDB,
		config:        TimescaleDBConfig{UseTimescaleDB: true},
		isTimescaleDB: false,
	}

	err = tsdb.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Check that TimescaleDB features are disabled
	if tsdb.isTimescaleDB {
		t.Error("Expected isTimescaleDB to be false, got true")
	}

	// Test case when TimescaleDB check fails with an unexpected error
	mockDB = NewMockDB()
	mockDB.RegisterQueryResult("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'", nil, errors.New("mocked query error"))
	tsdb = &TimescaleDB{
		Database:      mockDB,
		config:        TimescaleDBConfig{UseTimescaleDB: true},
		isTimescaleDB: false,
	}

	err = tsdb.Connect()
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestClose(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &TimescaleDB{
		Database: mockDB,
	}

	// Close should delegate to the underlying database
	err := tsdb.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Test error case
	mockDB = NewMockDB()
	mockDB.SetCloseError(errors.New("mocked close error"))
	tsdb = &TimescaleDB{
		Database: mockDB,
	}

	err = tsdb.Close()
	if err == nil {
		t.Error("Expected close error, got nil")
	}
}

func TestExtVersion(t *testing.T) {
	tsdb := &TimescaleDB{
		extVersion: "2.8.0",
	}

	if tsdb.ExtVersion() != "2.8.0" {
		t.Errorf("Expected ExtVersion() to return '2.8.0', got '%s'", tsdb.ExtVersion())
	}
}

func TestTimescaleDBInstance(t *testing.T) {
	tsdb := &TimescaleDB{
		isTimescaleDB: true,
	}

	if !tsdb.IsTimescaleDB() {
		t.Error("Expected IsTimescaleDB() to return true, got false")
	}

	tsdb.isTimescaleDB = false
	if tsdb.IsTimescaleDB() {
		t.Error("Expected IsTimescaleDB() to return false, got true")
	}
}

func TestApplyConfig(t *testing.T) {
	// Test when TimescaleDB is not available
	tsdb := &TimescaleDB{
		isTimescaleDB: false,
	}

	err := tsdb.ApplyConfig()
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test when TimescaleDB is available
	tsdb = &TimescaleDB{
		isTimescaleDB: true,
	}

	err = tsdb.ApplyConfig()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestExecuteSQLWithoutParams(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &TimescaleDB{
		Database: mockDB,
	}

	ctx := context.Background()

	// Test SELECT query
	mockResult := []map[string]interface{}{
		{"id": 1, "name": "Test"},
	}
	mockDB.RegisterQueryResult("SELECT * FROM test", mockResult, nil)

	result, err := tsdb.ExecuteSQLWithoutParams(ctx, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Test non-SELECT query (e.g., INSERT)
	insertResult, err := tsdb.ExecuteSQLWithoutParams(ctx, "INSERT INTO test (id, name) VALUES (1, 'Test')")
	if err != nil {
		t.Fatalf("Failed to execute statement: %v", err)
	}

	// Since the mock doesn't do much, just verify it's a MockResult
	_, ok := insertResult.(*MockResult)
	if !ok {
		t.Error("Expected result to be a MockResult")
	}

	// Test query error
	mockDB.RegisterQueryResult("SELECT * FROM error_table", nil, errors.New("mocked query error"))
	_, err = tsdb.ExecuteSQLWithoutParams(ctx, "SELECT * FROM error_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestExecuteSQL(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &TimescaleDB{
		Database: mockDB,
	}

	ctx := context.Background()

	// Test SELECT query with parameters
	mockResult := []map[string]interface{}{
		{"id": 1, "name": "Test"},
	}
	mockDB.RegisterQueryResult("SELECT * FROM test WHERE id = $1", mockResult, nil)

	result, err := tsdb.ExecuteSQL(ctx, "SELECT * FROM test WHERE id = $1", 1)
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Test non-SELECT query with parameters (e.g., INSERT)
	insertResult, err := tsdb.ExecuteSQL(ctx, "INSERT INTO test (id, name) VALUES ($1, $2)", 1, "Test")
	if err != nil {
		t.Fatalf("Failed to execute statement: %v", err)
	}

	// Since the mock doesn't do much, just verify it's a MockResult
	_, ok := insertResult.(*MockResult)
	if !ok {
		t.Error("Expected result to be a MockResult")
	}

	// Test query error with parameters
	mockDB.RegisterQueryResult("SELECT * FROM error_table WHERE id = $1", nil, errors.New("mocked query error"))
	_, err = tsdb.ExecuteSQL(ctx, "SELECT * FROM error_table WHERE id = $1", 1)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestIsSelectQuery(t *testing.T) {
	testCases := []struct {
		query    string
		expected bool
	}{
		{"SELECT * FROM test", true},
		{"select * from test", true},
		{"  SELECT * FROM test", true},
		{"\tSELECT * FROM test", true},
		{"\nSELECT * FROM test", true},
		{"INSERT INTO test VALUES (1)", false},
		{"UPDATE test SET name = 'Test'", false},
		{"DELETE FROM test", false},
		{"CREATE TABLE test (id INT)", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := isSelectQuery(tc.query)
		if result != tc.expected {
			t.Errorf("isSelectQuery(%q) = %v, expected %v", tc.query, result, tc.expected)
		}
	}
}
