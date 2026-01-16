package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOracleConnectionStringBuilder tests the Oracle connection string builder
func TestOracleConnectionStringBuilder(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "Basic connection with service name",
			config: Config{
				Type:        "oracle",
				Host:        "localhost",
				Port:        1521,
				User:        "testuser",
				Password:    "testpass",
				ServiceName: "TESTDB",
			},
			expected: "oracle://testuser:testpass@localhost:1521/TESTDB",
		},
		{
			name: "Connection with SID (legacy)",
			config: Config{
				Type:     "oracle",
				Host:     "localhost",
				Port:     1521,
				User:     "testuser",
				Password: "testpass",
				SID:      "ORCL",
			},
			expected: "oracle://testuser:testpass@localhost:1521/ORCL",
		},
		{
			name: "Cloud wallet connection",
			config: Config{
				Type:           "oracle",
				User:           "ADMIN",
				Password:       "pass123",
				ServiceName:    "mydb_high",
				WalletLocation: "/app/wallet",
			},
			expected: "oracle://ADMIN:pass123@mydb_high?wallet location=/app/wallet",
		},
		{
			name: "TNS entry connection",
			config: Config{
				Type:     "oracle",
				User:     "user",
				Password: "pass",
				TNSEntry: "PROD_DB",
				TNSAdmin: "/opt/oracle/admin",
			},
			expected: "oracle://user:pass@PROD_DB?tns admin=/opt/oracle/admin",
		},
		{
			name: "Connection with timeout and NLS settings",
			config: Config{
				Type:           "oracle",
				Host:           "localhost",
				Port:           1521,
				User:           "testuser",
				Password:       "testpass",
				ServiceName:    "TESTDB",
				ConnectTimeout: 30,
				NLSLang:        "AMERICAN_AMERICA.AL32UTF8",
			},
			expected: "oracle://testuser:testpass@localhost:1521/TESTDB?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connStr := buildOracleConnStr(tt.config)
			assert.Contains(t, connStr, tt.expected)
		})
	}
}

// TestOracleConnection tests actual Oracle database connection
func TestOracleConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if Oracle test database is available
	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := Config{
		Type:           "oracle",
		Host:           host,
		Port:           1521,
		User:           "testuser",
		Password:       "testpass",
		ServiceName:    "TESTDB",
		ConnectTimeout: 10,
		QueryTimeout:   30,
		MaxOpenConns:   5,
		MaxIdleConns:   2,
	}

	db, err := NewDatabase(config)
	require.NoError(t, err, "Failed to create Oracle database instance")
	require.NotNil(t, db)

	defer func() {
		_ = db.Close()
	}()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = db.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	err = db.Ping(ctx)
	require.NoError(t, err, "Failed to ping Oracle database")

	// Test basic query
	rows, err := db.Query(ctx, "SELECT 1 AS result FROM DUAL")
	require.NoError(t, err, "Failed to execute query")
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next(), "Expected at least one row")

	var result int
	err = rows.Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

// TestOracleSchemaQueries tests Oracle schema introspection
func TestOracleSchemaQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	db, err := NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	err = db.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()

	// Create test table
	_, err = db.Exec(ctx, `
		CREATE TABLE test_schema_query (
			id NUMBER(10) PRIMARY KEY,
			name VARCHAR2(100) NOT NULL,
			created_at DATE DEFAULT SYSDATE
		)
	`)
	require.NoError(t, err, "Failed to create test table")

	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE test_schema_query")
	}()

	// Test table listing
	rows, err := db.Query(ctx, "SELECT table_name FROM user_tables WHERE table_name = 'TEST_SCHEMA_QUERY'")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	require.True(t, rows.Next(), "Expected table to exist")

	var tableName string
	err = rows.Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "TEST_SCHEMA_QUERY", tableName)

	// Test column information
	rows, err = db.Query(ctx, `
		SELECT column_name, data_type, nullable 
		FROM user_tab_columns 
		WHERE table_name = 'TEST_SCHEMA_QUERY'
		ORDER BY column_id
	`)
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()

	columns := []string{}
	for rows.Next() {
		var colName, dataType, nullable string
		err = rows.Scan(&colName, &dataType, &nullable)
		require.NoError(t, err)
		columns = append(columns, colName)
	}

	assert.Equal(t, []string{"ID", "NAME", "CREATED_AT"}, columns)
}

// TestOracleTransactions tests Oracle transaction handling
func TestOracleTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	db, err := NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	err = db.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()

	// Setup test table
	_, _ = db.Exec(ctx, "DROP TABLE test_tx")
	_, err = db.Exec(ctx, "CREATE TABLE test_tx (id NUMBER, value VARCHAR2(50))")
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE test_tx")
	}()

	// Test transaction commit
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_tx (id, value) VALUES (1, 'committed')")
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Verify committed data
	var value string
	err = db.QueryRow(ctx, "SELECT value FROM test_tx WHERE id = 1").Scan(&value)
	require.NoError(t, err)
	assert.Equal(t, "committed", value)

	// Test transaction rollback
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO test_tx (id, value) VALUES (2, 'rolledback')")
	require.NoError(t, err)

	err = tx.Rollback()
	require.NoError(t, err)

	// Verify rolled back data doesn't exist
	rows, err := db.Query(ctx, "SELECT value FROM test_tx WHERE id = 2")
	require.NoError(t, err)
	defer func() {
		_ = rows.Close()
	}()
	assert.False(t, rows.Next(), "Expected no rows after rollback")
}

// TestOracleDataTypes tests various Oracle data types
func TestOracleDataTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("ORACLE_TEST_HOST")
	if host == "" {
		host = "localhost"
	}

	config := Config{
		Type:        "oracle",
		Host:        host,
		Port:        1521,
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	db, err := NewDatabase(config)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	err = db.Connect()
	if err != nil {
		t.Skipf("Skipping Oracle integration test: database not available (%v)", err)
		return
	}

	ctx := context.Background()

	// Create table with various data types
	_, _ = db.Exec(ctx, "DROP TABLE test_datatypes")
	_, err = db.Exec(ctx, `
		CREATE TABLE test_datatypes (
			id NUMBER(10),
			text_col VARCHAR2(100),
			number_col NUMBER(10,2),
			date_col DATE,
			timestamp_col TIMESTAMP
		)
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(ctx, "DROP TABLE test_datatypes")
	}()

	// Insert test data
	now := time.Now()
	_, err = db.Exec(ctx, `
		INSERT INTO test_datatypes (id, text_col, number_col, date_col, timestamp_col)
		VALUES (1, 'test string', 123.45, SYSDATE, SYSTIMESTAMP)
	`)
	require.NoError(t, err)

	// Query and verify data types
	var (
		id        int
		textCol   string
		numberCol float64
		dateCol   time.Time
		tsCol     time.Time
	)

	err = db.QueryRow(ctx, "SELECT id, text_col, number_col, date_col, timestamp_col FROM test_datatypes WHERE id = 1").
		Scan(&id, &textCol, &numberCol, &dateCol, &tsCol)
	require.NoError(t, err)

	assert.Equal(t, 1, id)
	assert.Equal(t, "test string", textCol)
	assert.InDelta(t, 123.45, numberCol, 0.01)
	assert.True(t, dateCol.Year() == now.Year())
	assert.True(t, tsCol.Year() == now.Year())
}

// TestOracleDefaults tests Oracle default configuration values
func TestOracleDefaults(t *testing.T) {
	config := Config{
		Type:        "oracle",
		Host:        "localhost",
		User:        "testuser",
		Password:    "testpass",
		ServiceName: "TESTDB",
	}

	config.SetDefaults()

	assert.Equal(t, 1521, config.Port, "Default port should be 1521")
	assert.Equal(t, "AMERICAN_AMERICA.AL32UTF8", config.NLSLang, "Default NLS_LANG should be UTF-8")
	assert.Equal(t, 50, config.MaxOpenConns, "Oracle should have higher max open connections")
	assert.Equal(t, 10, config.MaxIdleConns, "Oracle should have higher max idle connections")
	assert.Equal(t, 30*time.Minute, config.ConnMaxLifetime, "Oracle should have longer connection lifetime")
}

// TestOracleConnectionString tests the masked connection string output
func TestOracleConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "Standard connection",
			config: Config{
				Type:        "oracle",
				Host:        "localhost",
				Port:        1521,
				User:        "testuser",
				Password:    "secret",
				Name:        "TESTDB",
				ServiceName: "TESTDB",
			},
			expected: "oracle://testuser:***@localhost:1521/TESTDB",
		},
		{
			name: "Wallet connection",
			config: Config{
				Type:           "oracle",
				User:           "admin",
				Password:       "secret",
				ServiceName:    "mydb_high",
				WalletLocation: "/app/wallet",
			},
			expected: "oracle://admin:***@mydb_high (wallet: /app/wallet)",
		},
		{
			name: "TNS connection",
			config: Config{
				Type:     "oracle",
				User:     "user",
				Password: "secret",
				TNSEntry: "PROD_DB",
				TNSAdmin: "/opt/oracle/admin",
			},
			expected: "oracle://user:***@PROD_DB (TNS)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			db := &database{
				config:     tt.config,
				driverName: "oracle",
			}
			connStr := db.ConnectionString()
			assert.Equal(t, tt.expected, connStr)
		})
	}
}
