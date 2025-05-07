package mcp

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockDatabaseUseCase is a mock implementation of the database use case
type MockDatabaseUseCase struct {
	mock.Mock
}

// ExecuteStatement mocks the ExecuteStatement method
func (m *MockDatabaseUseCase) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, statement, params)
	return args.String(0), args.Error(1)
}

// GetDatabaseType mocks the GetDatabaseType method
func (m *MockDatabaseUseCase) GetDatabaseType(dbID string) (string, error) {
	args := m.Called(dbID)
	return args.String(0), args.Error(1)
}

// ExecuteQuery mocks the ExecuteQuery method
func (m *MockDatabaseUseCase) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, query, params)
	return args.String(0), args.Error(1)
}

// ExecuteTransaction mocks the ExecuteTransaction method
func (m *MockDatabaseUseCase) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	args := m.Called(ctx, dbID, action, txID, statement, params, readOnly)
	return args.String(0), args.Get(1).(map[string]interface{}), args.Error(2)
}

// GetDatabaseInfo mocks the GetDatabaseInfo method
func (m *MockDatabaseUseCase) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	args := m.Called(dbID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// ListDatabases mocks the ListDatabases method
func (m *MockDatabaseUseCase) ListDatabases() []string {
	args := m.Called()
	return args.Get(0).([]string)
}
