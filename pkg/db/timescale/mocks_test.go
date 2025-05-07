package timescale

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"testing"
)

// MockDB simulates a database for testing purposes
type MockDB struct {
	mockResults   map[string]MockExecuteResult
	lastQuery     string
	lastQueryArgs []interface{}
	queryHistory  []string
	connectCalled bool
	connectError  error
	closeCalled   bool
	closeError    error
	isTimescaleDB bool
	// Store the expected scan value for QueryRow
	queryScanValues map[string]interface{}
	// Added for additional mock methods
	queryResult []map[string]interface{}
	err         error
}

// MockExecuteResult is used for mocking query results
type MockExecuteResult struct {
	Result interface{}
	Error  error
}

// NewMockDB creates a new mock database for testing
func NewMockDB() *MockDB {
	return &MockDB{
		mockResults:     make(map[string]MockExecuteResult),
		queryScanValues: make(map[string]interface{}),
		queryHistory:    make([]string, 0),
		isTimescaleDB:   true, // Default is true
	}
}

// RegisterQueryResult registers a result to be returned for a specific query
func (m *MockDB) RegisterQueryResult(query string, result interface{}, err error) {
	// Store the result for exact matching
	m.mockResults[query] = MockExecuteResult{
		Result: result,
		Error:  err,
	}

	// Also store the result for partial matching
	if result != nil || err != nil {
		m.mockResults["partial:"+query] = MockExecuteResult{
			Result: result,
			Error:  err,
		}
	}

	// Also store the result as a scan value for QueryRow
	m.queryScanValues[query] = result
}

// getQueryResult tries to find a matching result for a query
// First tries exact match, then partial match
func (m *MockDB) getQueryResult(query string) (MockExecuteResult, bool) {
	// Try exact match first
	if result, ok := m.mockResults[query]; ok {
		return result, true
	}

	// Try partial match
	for k, v := range m.mockResults {
		if strings.HasPrefix(k, "partial:") && strings.Contains(query, k[8:]) {
			return v, true
		}
	}

	return MockExecuteResult{}, false
}

// GetLastQuery returns the last executed query and args
func (m *MockDB) GetLastQuery() (string, []interface{}) {
	return m.lastQuery, m.lastQueryArgs
}

// Connect implements db.Database.Connect
func (m *MockDB) Connect() error {
	m.connectCalled = true
	return m.connectError
}

// SetConnectError sets an error to be returned from Connect()
func (m *MockDB) SetConnectError(err error) {
	m.connectError = err
}

// Close implements db.Database.Close
func (m *MockDB) Close() error {
	m.closeCalled = true
	return m.closeError
}

// SetCloseError sets an error to be returned from Close()
func (m *MockDB) SetCloseError(err error) {
	m.closeError = err
}

// SetTimescaleAvailable sets whether TimescaleDB is available for this mock
func (m *MockDB) SetTimescaleAvailable(available bool) {
	m.isTimescaleDB = available
}

// Exec implements db.Database.Exec
func (m *MockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	m.lastQuery = query
	m.lastQueryArgs = args

	if result, found := m.getQueryResult(query); found {
		if result.Error != nil {
			return nil, result.Error
		}
		if sqlResult, ok := result.Result.(sql.Result); ok {
			return sqlResult, nil
		}
	}

	return &MockResult{}, nil
}

// Query implements db.Database.Query
func (m *MockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	m.lastQuery = query
	m.lastQueryArgs = args

	if result, found := m.getQueryResult(query); found {
		if result.Error != nil {
			return nil, result.Error
		}
		if rows, ok := result.Result.(*sql.Rows); ok {
			return rows, nil
		}
	}

	// Create a MockRows for the test
	return sql.OpenDB(&MockConnector{mockDB: m, query: query}).Query(query)
}

// QueryRow implements db.Database.QueryRow
func (m *MockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	m.lastQuery = query
	m.lastQueryArgs = args

	// Use a custom connector to create a sql.DB that's backed by our mock
	db := sql.OpenDB(&MockConnector{mockDB: m, query: query})
	return db.QueryRow(query)
}

// BeginTx implements db.Database.BeginTx
func (m *MockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, nil
}

// Prepare implements db.Database.Prepare
func (m *MockDB) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, nil
}

// Ping implements db.Database.Ping
func (m *MockDB) Ping(ctx context.Context) error {
	return nil
}

// DB implements db.Database.DB
func (m *MockDB) DB() *sql.DB {
	return nil
}

// ConnectionString implements db.Database.ConnectionString
func (m *MockDB) ConnectionString() string {
	return "mock://localhost/testdb"
}

// DriverName implements db.Database.DriverName
func (m *MockDB) DriverName() string {
	return "postgres"
}

// QueryTimeout implements db.Database.QueryTimeout
func (m *MockDB) QueryTimeout() int {
	return 30
}

// MockResult implements sql.Result
type MockResult struct{}

// LastInsertId implements sql.Result.LastInsertId
func (r *MockResult) LastInsertId() (int64, error) {
	return 1, nil
}

// RowsAffected implements sql.Result.RowsAffected
func (r *MockResult) RowsAffected() (int64, error) {
	return 1, nil
}

// MockTimescaleDB creates a TimescaleDB instance with mocked database for testing
func MockTimescaleDB(t testing.TB) (*DB, *MockDB) {
	mockDB := NewMockDB()
	mockDB.SetTimescaleAvailable(true)

	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
		extVersion:    "2.8.0",
		config: DBConfig{
			UseTimescaleDB: true,
		},
	}
	return tsdb, mockDB
}

// AssertQueryContains checks if the query contains the expected substrings
func AssertQueryContains(t testing.TB, query string, substrings ...string) {
	for _, substring := range substrings {
		if !contains(query, substring) {
			t.Errorf("Expected query to contain '%s', but got: %s", substring, query)
		}
	}
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Mock driver implementation to support sql.Row

// MockConnector implements driver.Connector
type MockConnector struct {
	mockDB *MockDB
	query  string
}

// Connect implements driver.Connector
func (c *MockConnector) Connect(_ context.Context) (driver.Conn, error) {
	return &MockConn{mockDB: c.mockDB, query: c.query}, nil
}

// Driver implements driver.Connector
func (c *MockConnector) Driver() driver.Driver {
	return &MockDriver{}
}

// MockDriver implements driver.Driver
type MockDriver struct{}

// Open implements driver.Driver
func (d *MockDriver) Open(name string) (driver.Conn, error) {
	return &MockConn{}, nil
}

// MockConn implements driver.Conn
type MockConn struct {
	mockDB *MockDB
	query  string
}

// Prepare implements driver.Conn
func (c *MockConn) Prepare(query string) (driver.Stmt, error) {
	return &MockStmt{mockDB: c.mockDB, query: query}, nil
}

// Close implements driver.Conn
func (c *MockConn) Close() error {
	return nil
}

// Begin implements driver.Conn
func (c *MockConn) Begin() (driver.Tx, error) {
	return nil, nil
}

// MockStmt implements driver.Stmt
type MockStmt struct {
	mockDB *MockDB
	query  string
}

// Close implements driver.Stmt
func (s *MockStmt) Close() error {
	return nil
}

// NumInput implements driver.Stmt
func (s *MockStmt) NumInput() int {
	return 0
}

// Exec implements driver.Stmt
func (s *MockStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, nil
}

// Query implements driver.Stmt
func (s *MockStmt) Query(args []driver.Value) (driver.Rows, error) {
	// Return the registered result for this query
	if s.mockDB != nil {
		if result, found := s.mockDB.getQueryResult(s.query); found {
			if result.Error != nil {
				return nil, result.Error
			}
			return &MockRows{value: result.Result}, nil
		}
	}
	return &MockRows{}, nil
}

// MockRows implements driver.Rows
type MockRows struct {
	value       interface{}
	currentRow  int
	columnNames []string
}

// Columns implements driver.Rows
func (r *MockRows) Columns() []string {
	// If we have a slice of maps, extract column names from the first map
	if rows, ok := r.value.([]map[string]interface{}); ok && len(rows) > 0 {
		if r.columnNames == nil {
			r.columnNames = make([]string, 0, len(rows[0]))
			for k := range rows[0] {
				r.columnNames = append(r.columnNames, k)
			}
		}
		return r.columnNames
	}
	return []string{"value"}
}

// Close implements driver.Rows
func (r *MockRows) Close() error {
	return nil
}

// Next implements driver.Rows
func (r *MockRows) Next(dest []driver.Value) error {
	// Handle slice of maps (multiple rows of data)
	if rows, ok := r.value.([]map[string]interface{}); ok {
		if r.currentRow < len(rows) {
			row := rows[r.currentRow]
			r.currentRow++

			// Find column values in the expected order
			columns := r.Columns()
			for i, col := range columns {
				if i < len(dest) {
					dest[i] = row[col]
				}
			}
			return nil
		}
		return io.EOF
	}

	// Handle simple string value
	if r.currentRow == 0 && r.value != nil {
		r.currentRow++
		dest[0] = r.value
		return nil
	}
	return io.EOF
}

// RunQueryTest executes a mock query test against the DB
func RunQueryTest(t *testing.T, testFunc func(*DB) error) {
	tsdb, _ := MockTimescaleDB(t)
	err := testFunc(tsdb)
	if err != nil {
		t.Errorf("Test failed: %v", err)
	}
}

// SetQueryResult sets the mock result for queries
func (m *MockDB) SetQueryResult(result []map[string]interface{}) {
	m.queryResult = result
}

// SetError sets the mock error
func (m *MockDB) SetError(errMsg string) {
	m.err = fmt.Errorf("%s", errMsg)
}

// LastQuery returns the last executed query
func (m *MockDB) LastQuery() string {
	return m.lastQuery
}

// QueryContains checks if the last query contains a substring
func (m *MockDB) QueryContains(substring string) bool {
	return strings.Contains(m.lastQuery, substring)
}

// ExecuteSQL implements db.Database.ExecuteSQL
func (m *MockDB) ExecuteSQL(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	m.lastQuery = query
	m.lastQueryArgs = args
	m.queryHistory = append(m.queryHistory, query)

	if m.err != nil {
		return nil, m.err
	}

	// If TimescaleDB is not available and the query is for TimescaleDB specific features
	if !m.isTimescaleDB && (strings.Contains(query, "time_bucket") ||
		strings.Contains(query, "hypertable") ||
		strings.Contains(query, "continuous_aggregate") ||
		strings.Contains(query, "timescaledb_information")) {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	if m.queryResult != nil {
		return m.queryResult, nil
	}

	// Return an empty result set by default
	return []map[string]interface{}{}, nil
}

// ExecuteSQLWithoutParams implements db.Database.ExecuteSQLWithoutParams
func (m *MockDB) ExecuteSQLWithoutParams(ctx context.Context, query string) (interface{}, error) {
	m.lastQuery = query
	m.lastQueryArgs = nil
	m.queryHistory = append(m.queryHistory, query)

	if m.err != nil {
		return nil, m.err
	}

	// If TimescaleDB is not available and the query is for TimescaleDB specific features
	if !m.isTimescaleDB && (strings.Contains(query, "time_bucket") ||
		strings.Contains(query, "hypertable") ||
		strings.Contains(query, "continuous_aggregate") ||
		strings.Contains(query, "timescaledb_information")) {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	if m.queryResult != nil {
		return m.queryResult, nil
	}

	// Return an empty result set by default
	return []map[string]interface{}{}, nil
}

// QueryHistory returns the complete history of queries executed
func (m *MockDB) QueryHistory() []string {
	return m.queryHistory
}

// AnyQueryContains checks if any query in the history contains the given substring
func (m *MockDB) AnyQueryContains(substring string) bool {
	for _, query := range m.queryHistory {
		if strings.Contains(query, substring) {
			return true
		}
	}
	return false
}
