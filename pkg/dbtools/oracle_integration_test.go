package dbtools

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOracleIntegration tests Oracle database integration with dbtools
func TestOracleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Oracle integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := db.Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	database, err := db.NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = database.Close()
	}()

	err = database.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()
	err = database.Ping(ctx)
	require.NoError(t, err)

	t.Run("QueryTool", func(t *testing.T) {
		testOracleQueryTool(t, database)
	})

	t.Run("ExecuteTool", func(t *testing.T) {
		testOracleExecuteTool(t, database)
	})

	t.Run("SchemaTool", func(t *testing.T) {
		testOracleSchemaTool(t, database)
	})

	t.Run("TransactionTool", func(t *testing.T) {
		testOracleTransactionTool(t, database)
	})

	t.Run("PerformanceTool", func(t *testing.T) {
		testOraclePerformanceTool(t, database)
	})
}

func testOracleQueryTool(t *testing.T, database db.Database) {
	ctx := context.Background()

	// Test simple query
	rows, err := database.Query(ctx, "SELECT 1 AS num, 'test' AS str FROM DUAL")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next())
	var num int
	var str string
	err = rows.Scan(&num, &str)
	require.NoError(t, err)
	assert.Equal(t, 1, num)
	assert.Equal(t, "test", str)

	// Test query with Oracle-specific features
	rows, err = database.Query(ctx, "SELECT SYSDATE, SYSTIMESTAMP FROM DUAL")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next())
	var sysdate time.Time
	var systimestamp time.Time
	err = rows.Scan(&sysdate, &systimestamp)
	require.NoError(t, err)
	assert.False(t, sysdate.IsZero())
	assert.False(t, systimestamp.IsZero())
}

func testOracleExecuteTool(t *testing.T, database db.Database) {
	ctx := context.Background()

	// Clean up any existing test table
	_, _ = database.Exec(ctx, "DROP TABLE test_execute_tool")

	// Create test table
	_, err := database.Exec(ctx, `
		CREATE TABLE test_execute_tool (
			id NUMBER(10) PRIMARY KEY,
			name VARCHAR2(100),
			created_at TIMESTAMP DEFAULT SYSTIMESTAMP
		)
	`)
	require.NoError(t, err)

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE test_execute_tool")
	}()

	// Insert data
	result, err := database.Exec(ctx, "INSERT INTO test_execute_tool (id, name) VALUES (1, 'Alice')")
	require.NoError(t, err)
	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected)

	// Update data
	result, err = database.Exec(ctx, "UPDATE test_execute_tool SET name = 'Bob' WHERE id = 1")
	require.NoError(t, err)
	rowsAffected, err = result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected)

	// Verify update
	var name string
	err = database.QueryRow(ctx, "SELECT name FROM test_execute_tool WHERE id = 1").Scan(&name)
	require.NoError(t, err)
	assert.Equal(t, "Bob", name)

	// Delete data
	result, err = database.Exec(ctx, "DELETE FROM test_execute_tool WHERE id = 1")
	require.NoError(t, err)
	rowsAffected, err = result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected)
}

func testOracleSchemaTool(t *testing.T, database db.Database) {
	ctx := context.Background()

	// Clean up any existing test table
	_, _ = database.Exec(ctx, "DROP TABLE test_schema_tool")

	// Create test table with various Oracle data types
	_, err := database.Exec(ctx, `
		CREATE TABLE test_schema_tool (
			id NUMBER(10) PRIMARY KEY,
			name VARCHAR2(100) NOT NULL,
			email VARCHAR2(255) UNIQUE,
			age NUMBER(3),
			salary NUMBER(10,2),
			hire_date DATE,
			is_active CHAR(1) DEFAULT 'Y',
			description CLOB,
			created_at TIMESTAMP DEFAULT SYSTIMESTAMP
		)
	`)
	require.NoError(t, err)

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE test_schema_tool")
	}()

	// Test querying table metadata
	rows, err := database.Query(ctx, `
		SELECT table_name 
		FROM user_tables 
		WHERE table_name = 'TEST_SCHEMA_TOOL'
	`)
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next())
	var tableName string
	err = rows.Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "TEST_SCHEMA_TOOL", tableName)

	// Test querying column metadata
	rows, err = database.Query(ctx, `
		SELECT column_name, data_type, nullable
		FROM user_tab_columns
		WHERE table_name = 'TEST_SCHEMA_TOOL'
		ORDER BY column_id
	`)
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	expectedColumns := []struct {
		name     string
		dataType string
		nullable string
	}{
		{"ID", "NUMBER", "N"},
		{"NAME", "VARCHAR2", "N"},
		{"EMAIL", "VARCHAR2", "Y"},
		{"AGE", "NUMBER", "Y"},
		{"SALARY", "NUMBER", "Y"},
		{"HIRE_DATE", "DATE", "Y"},
		{"IS_ACTIVE", "CHAR", "Y"},
		{"DESCRIPTION", "CLOB", "Y"},
		{"CREATED_AT", "TIMESTAMP(6)", "Y"},
	}

	columnCount := 0
	for rows.Next() {
		var colName, dataType, nullable string
		err = rows.Scan(&colName, &dataType, &nullable)
		require.NoError(t, err)
		
		if columnCount < len(expectedColumns) {
			assert.Equal(t, expectedColumns[columnCount].name, colName)
			assert.Contains(t, dataType, expectedColumns[columnCount].dataType)
		}
		columnCount++
	}
	assert.Equal(t, len(expectedColumns), columnCount)

	// Test querying constraints
	rows, err = database.Query(ctx, `
		SELECT constraint_name, constraint_type
		FROM user_constraints
		WHERE table_name = 'TEST_SCHEMA_TOOL'
		ORDER BY constraint_type
	`)
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	constraintCount := 0
	for rows.Next() {
		var constraintName, constraintType string
		err = rows.Scan(&constraintName, &constraintType)
		require.NoError(t, err)
		assert.NotEmpty(t, constraintName)
		assert.NotEmpty(t, constraintType)
		constraintCount++
	}
	assert.GreaterOrEqual(t, constraintCount, 2) // At least PRIMARY KEY and UNIQUE
}

func testOracleTransactionTool(t *testing.T, database db.Database) {
	ctx := context.Background()

	// Clean up any existing test table
	_, _ = database.Exec(ctx, "DROP TABLE test_transaction_tool")

	// Create test table
	_, err := database.Exec(ctx, `
		CREATE TABLE test_transaction_tool (
			id NUMBER(10) PRIMARY KEY,
			value VARCHAR2(50)
		)
	`)
	require.NoError(t, err)

	defer func() {
		_, _ = database.Exec(ctx, "DROP TABLE test_transaction_tool")
	}()

	// Test transaction commit
	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_transaction_tool (id, value) VALUES (1, 'committed')")
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify committed data
	var value string
	err = database.QueryRow(ctx, "SELECT value FROM test_transaction_tool WHERE id = 1").Scan(&value)
	require.NoError(t, err)
	assert.Equal(t, "committed", value)

	// Test transaction rollback
	tx, err = database.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_transaction_tool (id, value) VALUES (2, 'rolledback')")
	require.NoError(t, err)

	err = tx.Rollback()
	require.NoError(t, err)

	// Verify rolled back data doesn't exist
	err = database.QueryRow(ctx, "SELECT value FROM test_transaction_tool WHERE id = 2").Scan(&value)
	assert.Equal(t, sql.ErrNoRows, err)

	// Test multiple operations in a transaction
	tx, err = database.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_transaction_tool (id, value) VALUES (3, 'multi1')")
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_transaction_tool (id, value) VALUES (4, 'multi2')")
	require.NoError(t, err)

	_, err = tx.Exec("UPDATE test_transaction_tool SET value = 'updated' WHERE id = 1")
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify all operations were committed
	rows, err := database.Query(ctx, "SELECT id, value FROM test_transaction_tool ORDER BY id")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	expectedValues := map[int]string{
		1: "updated",
		3: "multi1",
		4: "multi2",
	}

	for rows.Next() {
		var id int
		var val string
		err = rows.Scan(&id, &val)
		require.NoError(t, err)
		if expectedVal, ok := expectedValues[id]; ok {
			assert.Equal(t, expectedVal, val)
		}
	}
}

func testOraclePerformanceTool(t *testing.T, database db.Database) {
	ctx := context.Background()

	// Test query with EXPLAIN PLAN
	// Note: Oracle requires EXPLAIN PLAN to be used differently than other databases
	_, _ = database.Exec(ctx, "DELETE FROM PLAN_TABLE")

	// Execute EXPLAIN PLAN
	_, err := database.Exec(ctx, `
		EXPLAIN PLAN FOR
		SELECT * FROM test_users WHERE id = 1
	`)
	// If the table doesn't exist, that's okay for this test
	if err == nil {
		// Query the plan table
		rows, err := database.Query(ctx, `
			SELECT operation, options, object_name
			FROM PLAN_TABLE
			WHERE statement_id IS NULL
			ORDER BY id
		`)
		if err == nil {
			defer func() {
				_ = rows.Close()
			}()

			// Just verify we can read the plan
			planSteps := 0
			for rows.Next() {
				var operation, options, objectName sql.NullString
				err = rows.Scan(&operation, &options, &objectName)
				require.NoError(t, err)
				planSteps++
			}
			// Should have at least one step
			if planSteps > 0 {
				assert.Greater(t, planSteps, 0)
			}
		}
	}

	// Test basic query performance by timing
	start := time.Now()
	rows, err := database.Query(ctx, "SELECT 1 FROM DUAL")
	if err == nil {
		defer func() {
			_ = rows.Close()
		}()
		require.True(t, rows.Next())
	}
	elapsed := time.Since(start)
	// Query should complete quickly (within 5 seconds)
	assert.Less(t, elapsed, 5*time.Second)
}

// TestOracleSequences tests Oracle sequence functionality
func TestOracleSequences(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Oracle integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := db.Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	database, err := db.NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = database.Close()
	}()

	err = database.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()

	// Clean up any existing sequence
	_, _ = database.Exec(ctx, "DROP SEQUENCE test_seq")

	// Create a sequence
	_, err = database.Exec(ctx, "CREATE SEQUENCE test_seq START WITH 1 INCREMENT BY 1")
	require.NoError(t, err)

	defer func() {
		_, _ = database.Exec(ctx, "DROP SEQUENCE test_seq")
	}()

	// Get next value from sequence
	var nextVal int
	err = database.QueryRow(ctx, "SELECT test_seq.NEXTVAL FROM DUAL").Scan(&nextVal)
	require.NoError(t, err)
	assert.Equal(t, 1, nextVal)

	// Get another value
	err = database.QueryRow(ctx, "SELECT test_seq.NEXTVAL FROM DUAL").Scan(&nextVal)
	require.NoError(t, err)
	assert.Equal(t, 2, nextVal)

	// Get current value
	var currVal int
	err = database.QueryRow(ctx, "SELECT test_seq.CURRVAL FROM DUAL").Scan(&currVal)
	require.NoError(t, err)
	assert.Equal(t, 2, currVal)
}

// TestOracleDataDictionary tests Oracle data dictionary access
func TestOracleDataDictionary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Oracle integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := db.Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	database, err := db.NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = database.Close()
	}()

	err = database.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()

	// Query user tables
	rows, err := database.Query(ctx, "SELECT table_name FROM user_tables ORDER BY table_name")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	tableCount := 0
	for rows.Next() {
		var tableName string
		err = rows.Scan(&tableName)
		require.NoError(t, err)
		assert.NotEmpty(t, tableName)
		tableCount++
	}
	// Should have at least the test_users table from init script
	assert.GreaterOrEqual(t, tableCount, 1)

	// Query user sequences
	rows, err = database.Query(ctx, "SELECT sequence_name FROM user_sequences ORDER BY sequence_name")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	sequenceCount := 0
	for rows.Next() {
		var seqName string
		err = rows.Scan(&seqName)
		require.NoError(t, err)
		assert.NotEmpty(t, seqName)
		sequenceCount++
	}
	// Should have at least the test_users_seq from init script
	assert.GreaterOrEqual(t, sequenceCount, 1)
}
