package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewDatabase_MySQLDSNIncludesQueryTimeout locks in the fix for
// FreePeak/db-mcp-server issue #29: the user reported MySQL queries
// hitting a 30-second deadline even though they had set a custom
// query_timeout in config. The reason was that the MySQL DSN did not
// propagate the configured timeouts to the driver; only the default
// go-sql-driver/mysql 30s timeout applied.
//
// The fix wires ConnectTimeout and QueryTimeout into the MySQL DSN as
// `timeout`, `readTimeout`, and `writeTimeout` query parameters, which
// the driver uses to bound both connection establishment and per-query
// read/write operations.
func TestNewDatabase_MySQLDSNIncludesQueryTimeout(t *testing.T) {
	cfg := Config{
		Type:           "mysql",
		Host:           "localhost",
		Port:           3306,
		User:           "user",
		Password:       "password",
		Name:           "testdb",
		ConnectTimeout: 5,
		QueryTimeout:   120,
	}
	// SetDefaults would set QueryTimeout to 30; reset after defaults to keep
	// our explicit value intact for the DSN test.
	cfg.SetDefaults()
	cfg.ConnectTimeout = 5
	cfg.QueryTimeout = 120

	db, err := NewDatabase(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	connStr := db.ConnectionString()
	assert.Contains(t, connStr, "timeout=5s", "DSN must include connect timeout (issue #29)")
	assert.Contains(t, connStr, "readTimeout=120s", "DSN must include read timeout derived from query_timeout")
	assert.Contains(t, connStr, "writeTimeout=120s", "DSN must include write timeout derived from query_timeout")
}

// TestNewDatabase_MySQLDSNNoTimeoutWhenUnset ensures the DSN still surfaces
// driver defaults when ConnectTimeout is left at zero. NewDatabase runs
// SetDefaults which populates ConnectTimeout=10 and QueryTimeout=30, so the
// masked DSN will include those values; the contract is that the operator's
// explicit zero is honored only when the defaults are disabled.
func TestNewDatabase_MySQLDSNNoTimeoutWhenUnset(t *testing.T) {
	cfg := Config{
		Type: "mysql",
		Host: "localhost",
		Port: 3306,
		User: "user",
		Name: "testdb",
	}
	db, err := NewDatabase(cfg)
	assert.NoError(t, err)
	connStr := db.ConnectionString()
	// The defaults are 10s connect / 30s query; assert those surface in the
	// masked DSN so operators can see the effective timeout values.
	assert.Contains(t, connStr, "timeout=10s", "DSN must surface default connect timeout")
	assert.Contains(t, connStr, "readTimeout=30s", "DSN must surface default query timeout")
	assert.Contains(t, connStr, "writeTimeout=30s", "DSN must surface default query timeout")
}
