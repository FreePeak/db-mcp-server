package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegressionAllDatabases runs comprehensive regression tests across all database types
func TestRegressionAllDatabases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping regression tests in short mode")
	}

	tests := []struct {
		name         string
		config       Config
		skipIfNoHost bool
	}{
		{
			name: "MySQL",
			config: Config{
				Type:     "mysql",
				Host:     getEnvOrDefault("MYSQL_TEST_HOST", "localhost"),
				Port:     3306,
				User:     "user1",
				Password: "password1",
				Name:     "db1",
			},
			skipIfNoHost: false,
		},
		{
			name: "PostgreSQL",
			config: Config{
				Type:     "postgres",
				Host:     getEnvOrDefault("POSTGRES_TEST_HOST", "localhost"),
				Port:     5432,
				User:     "user1",
				Password: "password1",
				Name:     "db1",
			},
			skipIfNoHost: false,
		},
		{
			name: "SQLite",
			config: Config{
				Type:         "sqlite",
				DatabasePath: ":memory:",
			},
			skipIfNoHost: false,
		},
		{
			name: "Oracle",
			config: Config{
				Type:        "oracle",
				Host:        getEnvOrDefault("ORACLE_TEST_HOST", "localhost"),
				Port:        1521,
				User:        "testuser",
				Password:    "testpass",
				ServiceName: "TESTDB",
			},
			skipIfNoHost: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, err := NewDatabase(tt.config)
			require.NoError(t, err, "Failed to create database instance")

			err = database.Connect()
			if err != nil {
				if tt.skipIfNoHost {
					t.Skipf("Skipping %s test: database not available (%v)", tt.name, err)
					return
				}
				require.NoError(t, err, "Failed to connect to database")
			}

			defer func() {
				_ = database.Close()
			}()

			ctx := context.Background()

			// Test 1: Ping
			err = database.Ping(ctx)
			require.NoError(t, err, "Ping failed")

			// Test 2: Basic Query
			testBasicQuery(t, database, tt.config.Type)

			// Test 3: Execute Operations
			testExecuteOperations(t, database, tt.config.Type)

			// Test 4: Transaction Support
			testTransactionSupport(t, database, tt.config.Type)

			// Test 5: Data Types
			testDataTypeSupport(t, database, tt.config.Type)
		})
	}
}

func testBasicQuery(t *testing.T, database Database, dbType string) {
	ctx := context.Background()

	var query string
	switch dbType {
	case "mysql":
		query = "SELECT 1 AS num, 'test' AS str"
	case "postgres":
		query = "SELECT 1 AS num, 'test' AS str"
	case "sqlite":
		query = "SELECT 1 AS num, 'test' AS str"
	case "oracle":
		query = "SELECT 1 AS num, 'test' AS str FROM DUAL"
	default:
		t.Skipf("Unknown database type: %s", dbType)
		return
	}

	rows, err := database.Query(ctx, query)
	require.NoError(t, err, "Query failed")
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next(), "Expected at least one row")

	var num int
	var str string
	err = rows.Scan(&num, &str)
	require.NoError(t, err, "Scan failed")
	assert.Equal(t, 1, num)
	assert.Equal(t, "test", str)
}

func testExecuteOperations(t *testing.T, database Database, dbType string) {
	ctx := context.Background()

	tableName := "regression_test_table"

	// Drop table if exists (different syntax per database)
	switch dbType {
	case "mysql", "postgres", "sqlite":
		_, _ = database.Exec(ctx, "DROP TABLE IF EXISTS "+tableName)
	case "oracle":
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}

	// Create table with appropriate syntax
	var createSQL string
	switch dbType {
	case "mysql", "postgres":
		createSQL = "CREATE TABLE " + tableName + " (id INT PRIMARY KEY, name VARCHAR(100), value INT)"
	case "sqlite":
		createSQL = "CREATE TABLE " + tableName + " (id INTEGER PRIMARY KEY, name TEXT, value INTEGER)"
	case "oracle":
		createSQL = "CREATE TABLE " + tableName + " (id NUMBER(10) PRIMARY KEY, name VARCHAR2(100), value NUMBER(10))"
	default:
		t.Skipf("Unknown database type: %s", dbType)
		return
	}

	_, err := database.Exec(ctx, createSQL)
	require.NoError(t, err, "Create table failed")

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}()

	// Insert data - same SQL works for all database types
	insertSQL := "INSERT INTO " + tableName + " (id, name, value) VALUES (1, 'test', 100)"
	result, err := database.Exec(ctx, insertSQL)
	require.NoError(t, err, "Insert failed")

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err, "RowsAffected failed")
	assert.Equal(t, int64(1), rowsAffected)

	// Update data
	updateSQL := "UPDATE " + tableName + " SET value = 200 WHERE id = 1"
	result, err = database.Exec(ctx, updateSQL)
	require.NoError(t, err, "Update failed")

	rowsAffected, err = result.RowsAffected()
	require.NoError(t, err, "RowsAffected failed")
	assert.Equal(t, int64(1), rowsAffected)

	// Verify update
	var value int
	querySQL := "SELECT value FROM " + tableName + " WHERE id = 1"
	err = database.QueryRow(ctx, querySQL).Scan(&value)
	require.NoError(t, err, "Select failed")
	assert.Equal(t, 200, value)

	// Delete data
	deleteSQL := "DELETE FROM " + tableName + " WHERE id = 1"
	result, err = database.Exec(ctx, deleteSQL)
	require.NoError(t, err, "Delete failed")

	rowsAffected, err = result.RowsAffected()
	require.NoError(t, err, "RowsAffected failed")
	assert.Equal(t, int64(1), rowsAffected)
}

func testTransactionSupport(t *testing.T, database Database, dbType string) {
	ctx := context.Background()

	tableName := "regression_test_tx"

	// Drop and create table
	switch dbType {
	case "mysql", "postgres", "sqlite":
		_, _ = database.Exec(ctx, "DROP TABLE IF EXISTS "+tableName)
	case "oracle":
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}

	var createSQL string
	switch dbType {
	case "mysql", "postgres":
		createSQL = "CREATE TABLE " + tableName + " (id INT PRIMARY KEY, value VARCHAR(50))"
	case "sqlite":
		createSQL = "CREATE TABLE " + tableName + " (id INTEGER PRIMARY KEY, value TEXT)"
	case "oracle":
		createSQL = "CREATE TABLE " + tableName + " (id NUMBER(10) PRIMARY KEY, value VARCHAR2(50))"
	default:
		t.Skipf("Unknown database type: %s", dbType)
		return
	}

	_, err := database.Exec(ctx, createSQL)
	require.NoError(t, err, "Create table failed")

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}()

	// Test commit
	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err, "BeginTx failed")

	insertSQL := "INSERT INTO " + tableName + " (id, value) VALUES (1, 'committed')"
	_, err = tx.Exec(insertSQL)
	require.NoError(t, err, "Insert in transaction failed")

	err = tx.Commit()
	require.NoError(t, err, "Commit failed")

	// Verify committed data
	var value string
	querySQL := "SELECT value FROM " + tableName + " WHERE id = 1"
	err = database.QueryRow(ctx, querySQL).Scan(&value)
	require.NoError(t, err, "Select after commit failed")
	assert.Equal(t, "committed", value)

	// Test rollback
	tx, err = database.BeginTx(ctx, nil)
	require.NoError(t, err, "BeginTx failed")

	insertSQL = "INSERT INTO " + tableName + " (id, value) VALUES (2, 'rolledback')"
	_, err = tx.Exec(insertSQL)
	require.NoError(t, err, "Insert in transaction failed")

	err = tx.Rollback()
	require.NoError(t, err, "Rollback failed")

	// Verify rolled back data doesn't exist
	querySQL = "SELECT value FROM " + tableName + " WHERE id = 2"
	rows, err := database.Query(ctx, querySQL)
	require.NoError(t, err, "Query after rollback failed")
	defer func() {
		_ = rows.Close()
	}()
	assert.False(t, rows.Next(), "Expected no rows after rollback")
}

func testDataTypeSupport(t *testing.T, database Database, dbType string) {
	ctx := context.Background()

	tableName := "regression_test_datatypes"

	// Drop table if exists
	switch dbType {
	case "mysql", "postgres", "sqlite":
		_, _ = database.Exec(ctx, "DROP TABLE IF EXISTS "+tableName)
	case "oracle":
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}

	// Create table with various data types
	// Note: MySQL and PostgreSQL have slight differences in data types but we test them separately
	var createSQL string
	switch dbType {
	case "mysql":
		createSQL = `CREATE TABLE ` + tableName + ` (
			id INT PRIMARY KEY,
			text_col VARCHAR(100),
			int_col INT,
			float_col DECIMAL(10,2),
			date_col DATETIME,
			bool_col BOOLEAN
		)`
	case "postgres":
		createSQL = `CREATE TABLE ` + tableName + ` (
			id INT PRIMARY KEY,
			text_col VARCHAR(100),
			int_col INT,
			float_col DECIMAL(10,2),
			date_col TIMESTAMP,
			bool_col BOOLEAN
		)`
	case "sqlite":
		createSQL = `CREATE TABLE ` + tableName + ` (
			id INTEGER PRIMARY KEY,
			text_col TEXT,
			int_col INTEGER,
			float_col REAL,
			date_col TEXT,
			bool_col INTEGER
		)`
	case "oracle":
		createSQL = `CREATE TABLE ` + tableName + ` (
			id NUMBER(10) PRIMARY KEY,
			text_col VARCHAR2(100),
			int_col NUMBER(10),
			float_col NUMBER(10,2),
			date_col TIMESTAMP,
			bool_col CHAR(1)
		)`
	default:
		t.Skipf("Unknown database type: %s", dbType)
		return
	}

	_, err := database.Exec(ctx, createSQL)
	require.NoError(t, err, "Create table failed")

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE "+tableName)
	}()

	// Insert test data
	var insertSQL string
	switch dbType {
	case "mysql":
		insertSQL = "INSERT INTO " + tableName + " (id, text_col, int_col, float_col, date_col, bool_col) VALUES (1, 'test', 123, 456.78, NOW(), 1)"
	case "postgres":
		insertSQL = "INSERT INTO " + tableName + " (id, text_col, int_col, float_col, date_col, bool_col) VALUES (1, 'test', 123, 456.78, NOW(), true)"
	case "sqlite":
		insertSQL = "INSERT INTO " + tableName + " (id, text_col, int_col, float_col, date_col, bool_col) VALUES (1, 'test', 123, 456.78, datetime('now'), 1)"
	case "oracle":
		insertSQL = "INSERT INTO " + tableName + " (id, text_col, int_col, float_col, date_col, bool_col) VALUES (1, 'test', 123, 456.78, SYSTIMESTAMP, '1')"
	}

	_, err = database.Exec(ctx, insertSQL)
	require.NoError(t, err, "Insert failed")

	// Query and verify data
	querySQL := "SELECT text_col, int_col, float_col FROM " + tableName + " WHERE id = 1"
	var textCol string
	var intCol int
	var floatCol float64
	err = database.QueryRow(ctx, querySQL).Scan(&textCol, &intCol, &floatCol)
	require.NoError(t, err, "Select failed")

	assert.Equal(t, "test", textCol)
	assert.Equal(t, 123, intCol)
	assert.InDelta(t, 456.78, floatCol, 0.01)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestConnectionPooling tests database connection pooling for all databases
func TestConnectionPooling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection pooling tests in short mode")
	}

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "MySQL Connection Pooling",
			config: Config{
				Type:         "mysql",
				Host:         getEnvOrDefault("MYSQL_TEST_HOST", "localhost"),
				Port:         3306,
				User:         "user1",
				Password:     "password1",
				Name:         "db1",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
		{
			name: "PostgreSQL Connection Pooling",
			config: Config{
				Type:         "postgres",
				Host:         getEnvOrDefault("POSTGRES_TEST_HOST", "localhost"),
				Port:         5432,
				User:         "user1",
				Password:     "password1",
				Name:         "db1",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
		{
			name: "Oracle Connection Pooling",
			config: Config{
				Type:         "oracle",
				Host:         getEnvOrDefault("ORACLE_TEST_HOST", "localhost"),
				Port:         1521,
				User:         "testuser",
				Password:     "testpass",
				ServiceName:  "TESTDB",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, err := NewDatabase(tt.config)
			require.NoError(t, err)

			err = database.Connect()
			if err != nil {
				t.Skipf("Skipping test: database not available (%v)", err)
				return
			}

			defer func() {
				_ = database.Close()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Perform multiple concurrent queries to test connection pooling
			done := make(chan bool, 5)
			for i := 0; i < 5; i++ {
				go func() {
					var query string
					if tt.config.Type == "oracle" {
						query = "SELECT 1 FROM DUAL"
					} else {
						query = "SELECT 1"
					}

					rows, err := database.Query(ctx, query)
					if err == nil {
						_ = rows.Close()
					}
					done <- true
				}()
			}

			// Wait for all queries to complete
			for i := 0; i < 5; i++ {
				<-done
			}

			// Verify database is still accessible
			err = database.Ping(ctx)
			assert.NoError(t, err)
		})
	}
}
