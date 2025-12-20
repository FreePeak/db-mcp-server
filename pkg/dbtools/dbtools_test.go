package dbtools

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDB is a mock implementation of the db.Database interface
type MockDB struct {
	mock.Mock
}

func (m *MockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	callArgs := []interface{}{ctx, query}
	callArgs = append(callArgs, args...)
	args1 := m.Called(callArgs...)
	return args1.Get(0).(*sql.Rows), args1.Error(1)
}

func (m *MockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	callArgs := []interface{}{ctx, query}
	callArgs = append(callArgs, args...)
	args1 := m.Called(callArgs...)
	return args1.Get(0).(*sql.Row)
}

func (m *MockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	callArgs := []interface{}{ctx, query}
	callArgs = append(callArgs, args...)
	args1 := m.Called(callArgs...)
	return args1.Get(0).(sql.Result), args1.Error(1)
}

func (m *MockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	args1 := m.Called(ctx, opts)
	return args1.Get(0).(*sql.Tx), args1.Error(1)
}

func (m *MockDB) Connect() error {
	args1 := m.Called()
	return args1.Error(0)
}

func (m *MockDB) Close() error {
	args1 := m.Called()
	return args1.Error(0)
}

func (m *MockDB) Ping(ctx context.Context) error {
	args1 := m.Called(ctx)
	return args1.Error(0)
}

func (m *MockDB) DriverName() string {
	args1 := m.Called()
	return args1.String(0)
}

func (m *MockDB) ConnectionString() string {
	args1 := m.Called()
	return args1.String(0)
}

func (m *MockDB) DB() *sql.DB {
	args1 := m.Called()
	return args1.Get(0).(*sql.DB)
}

// MockRows implements a mock sql.Rows
type MockRows struct {
	mock.Mock
}

func (m *MockRows) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRows) Columns() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRows) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	args := m.Called(dest)
	return args.Error(0)
}

func (m *MockRows) Err() error {
	args := m.Called()
	return args.Error(0)
}

// MockResult implements a mock sql.Result
type MockResult struct {
	mock.Mock
}

func (m *MockResult) LastInsertId() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockResult) RowsAffected() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

// TestQuery tests the Query function
func TestQuery(t *testing.T) {
	// Setup mock
	mockDB := new(MockDB)

	// Use nil for rows since we can't easily create a real *sql.Rows
	var nilRows *sql.Rows

	ctx := context.Background()
	sqlQuery := "SELECT * FROM test_table WHERE id = ?"
	args := []interface{}{1}

	// Mock expectations
	mockDB.On("Query", ctx, sqlQuery, args[0]).Return(nilRows, nil)

	// Call function under test
	rows, err := Query(ctx, mockDB, sqlQuery, args...)

	// Assertions
	assert.NoError(t, err)
	assert.Nil(t, rows)
	mockDB.AssertExpectations(t)
}

// TestQueryWithError tests the Query function with an error
func TestQueryWithError(t *testing.T) {
	// Setup mock
	mockDB := new(MockDB)
	expectedErr := errors.New("database error")

	ctx := context.Background()
	sqlQuery := "SELECT * FROM test_table WHERE id = ?"
	args := []interface{}{1}

	// Mock expectations
	mockDB.On("Query", ctx, sqlQuery, args[0]).Return((*sql.Rows)(nil), expectedErr)

	// Call function under test
	rows, err := Query(ctx, mockDB, sqlQuery, args...)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, rows)
	mockDB.AssertExpectations(t)
}

// TestExec tests the Exec function
func TestExec(t *testing.T) {
	// Setup mock
	mockDB := new(MockDB)
	mockResult := new(MockResult)

	ctx := context.Background()
	sqlQuery := "INSERT INTO test_table (name) VALUES (?)"
	args := []interface{}{"test"}

	// Mock expectations
	mockResult.On("LastInsertId").Return(int64(1), nil)
	mockResult.On("RowsAffected").Return(int64(1), nil)
	mockDB.On("Exec", ctx, sqlQuery, args[0]).Return(mockResult, nil)

	// Call function under test
	result, err := Exec(ctx, mockDB, sqlQuery, args...)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, mockResult, result)

	// Verify the result
	id, err := result.LastInsertId()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	affected, err := result.RowsAffected()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	mockDB.AssertExpectations(t)
	mockResult.AssertExpectations(t)
}

// TODO: Add tests for showConnectedDatabases
// Note: Testing showConnectedDatabases requires proper mocking of the database manager
// and related functions. This should be implemented with proper dependency injection
// to make the function more testable without having to rely on global variables.
//
// The test should verify:
// 1. That connected databases are correctly reported with status "connected"
// 2. That failed database connections are reported with status "disconnected"
// 3. That latency measurements are included in the response
// 4. That it works with multiple database connections
