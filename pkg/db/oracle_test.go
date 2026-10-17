package db

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sijms/go-ora/v2/configurations"
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

// TestOracleUserInfoEncoding verifies oracleUserInfo percent-encodes the
// userinfo section so URL delimiters in credentials cannot corrupt the DSN
// (see issue #106, "Cannot connect to oracle db when '#' is in password").
func TestOracleUserInfoEncoding(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		password string
		expected string
	}{
		{
			name:     "plain password stays byte-identical",
			user:     "testuser",
			password: "testpass",
			expected: "testuser:testpass",
		},
		{
			name:     "hash in password",
			user:     "scott",
			password: "pa#ss",
			expected: "scott:pa%23ss",
		},
		{
			name:     "at sign, slash, percent and question mark",
			user:     "scott",
			password: "p@ss/w%rd?x",
			expected: "scott:p%40ss%2Fw%25rd%3Fx",
		},
		{
			name:     "space and colon",
			user:     "scott",
			password: "sp ace:8",
			expected: "scott:sp%20ace%3A8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, oracleUserInfo(tt.user, tt.password))
		})
	}
}

// TestOracleConnStrSpecialCharPasswordStandard verifies that the standard
// host:port/service Oracle DSN round-trips through go-ora's own parser when
// the password contains characters that are special in a URL.
func TestOracleConnStrSpecialCharPasswordStandard(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "hash", password: "pa#ss"},
		{name: "at sign, slash, percent and question mark", password: "p@ss/w%rd?x"},
		{name: "space and colon", password: "sp ace:8"},
		{name: "plain", password: "testpass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Type:        "oracle",
				Host:        "localhost",
				Port:        1521,
				User:        "scott",
				Password:    tt.password,
				ServiceName: "ORCL",
			}

			connStr := buildOracleConnStr(config)

			cfg, err := configurations.ParseConfig(connStr)
			require.NoError(t, err, "go-ora must accept the generated DSN: %s", connStr)
			assert.Equal(t, "scott", cfg.UserID)
			assert.Equal(t, tt.password, cfg.Password)
			assert.Equal(t, "ORCL", cfg.ServiceName)
			require.Len(t, cfg.Servers, 1)
			assert.Equal(t, "localhost", cfg.Servers[0].Addr)
			assert.Equal(t, 1521, cfg.Servers[0].Port)
		})
	}
}

// TestOracleConnStrSpecialCharPasswordTNSAndWallet verifies the TNS and
// wallet DSN variants keep credentials and query options intact when the
// password contains special characters. These variants have a port-less
// host, which go-ora's ParseConfig rejects for unrelated reasons, so the
// assertions are made against net/url instead.
func TestOracleConnStrSpecialCharPasswordTNSAndWallet(t *testing.T) {
	const specialPassword = "pa#ss@1/2?3"

	t.Run("TNS entry", func(t *testing.T) {
		config := Config{
			Type:     "oracle",
			User:     "scott",
			Password: specialPassword,
			TNSEntry: "PROD_DB",
			TNSAdmin: "/opt/oracle/admin",
		}

		connStr := buildOracleConnStr(config)

		u, err := url.Parse(connStr)
		require.NoError(t, err, "generated DSN must be a valid URL: %s", connStr)
		password, ok := u.User.Password()
		require.True(t, ok)
		assert.Equal(t, specialPassword, password)
		assert.Equal(t, "scott", u.User.Username())
		assert.Equal(t, "PROD_DB", u.Host)
		assert.Equal(t, "/opt/oracle/admin", u.Query().Get("tns admin"))
	})

	t.Run("wallet", func(t *testing.T) {
		config := Config{
			Type:           "oracle",
			User:           "ADMIN",
			Password:       specialPassword,
			ServiceName:    "mydb_high",
			WalletLocation: "/app/wallet",
		}

		connStr := buildOracleConnStr(config)

		u, err := url.Parse(connStr)
		require.NoError(t, err, "generated DSN must be a valid URL: %s", connStr)
		password, ok := u.User.Password()
		require.True(t, ok)
		assert.Equal(t, specialPassword, password)
		assert.Equal(t, "ADMIN", u.User.Username())
		assert.Equal(t, "mydb_high", u.Host)
		assert.Equal(t, "/app/wallet", u.Query().Get("wallet location"))
	})
}

// TestOracleSpecialCharPasswordRegression pins the failure mode fixed for
// issue #106: a DSN assembled by raw string concatenation cannot survive a
// '#' in the password, while the generated one parses cleanly.
func TestOracleSpecialCharPasswordRegression(t *testing.T) {
	// Deliberately invalid: the raw unencoded DSN is the regression subject.
	_, err := url.Parse("oracle://scott:pa#ss@localhost:1521/ORCL") //nolint:staticcheck // testing that this exact malformed URL fails
	require.Error(t, err, "raw unencoded '#' DSN is expected to be unparseable")

	config := Config{
		Type:        "oracle",
		Host:        "localhost",
		Port:        1521,
		User:        "scott",
		Password:    "pa#ss",
		ServiceName: "ORCL",
	}

	_, err = configurations.ParseConfig(buildOracleConnStr(config))
	require.NoError(t, err, "generated DSN with '#' password must parse")
}
