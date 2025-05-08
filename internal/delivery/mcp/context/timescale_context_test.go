package context_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

// MockDatabaseUseCase is a mock implementation of the UseCaseProvider interface
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

func TestTimescaleDBContextProvider(t *testing.T) {
	// Create a mock use case provider
	mockUseCase := new(MockDatabaseUseCase)

	// Create a context for testing
	ctx := context.Background()

	t.Run("detect_timescaledb_with_extension", func(t *testing.T) {
		// Sample result indicating TimescaleDB is available
		sampleVersionResult := `[{"extversion": "2.9.1"}]`

		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "test_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "test_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(sampleVersionResult, nil).Once()

		// Create the context provider
		provider := mcp.NewTimescaleDBContextProvider()

		// Call the detection method
		contextInfo, err := provider.DetectTimescaleDB(ctx, "test_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, contextInfo)
		assert.True(t, contextInfo.IsTimescaleDB)
		assert.Equal(t, "2.9.1", contextInfo.Version)

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("detect_timescaledb_with_no_extension", func(t *testing.T) {
		// Sample result indicating TimescaleDB is not available
		sampleEmptyResult := `[]`

		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "postgres_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "postgres_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(sampleEmptyResult, nil).Once()

		// Create the context provider
		provider := mcp.NewTimescaleDBContextProvider()

		// Call the detection method
		contextInfo, err := provider.DetectTimescaleDB(ctx, "postgres_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, contextInfo)
		assert.False(t, contextInfo.IsTimescaleDB)
		assert.Empty(t, contextInfo.Version)

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("detect_timescaledb_with_non_postgres", func(t *testing.T) {
		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "mysql_db").Return("mysql", nil).Once()

		// Create the context provider
		provider := mcp.NewTimescaleDBContextProvider()

		// Call the detection method
		contextInfo, err := provider.DetectTimescaleDB(ctx, "mysql_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, contextInfo)
		assert.False(t, contextInfo.IsTimescaleDB)
		assert.Empty(t, contextInfo.Version)

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})

	t.Run("get_hypertables_info", func(t *testing.T) {
		// Sample result with list of hypertables
		sampleHypertablesResult := `[
			{"table_name": "metrics", "time_column": "timestamp", "chunk_interval": "1 day"},
			{"table_name": "logs", "time_column": "log_time", "chunk_interval": "4 hours"}
		]`

		// Set up expectations for the mock
		mockUseCase.On("GetDatabaseType", "timescale_db").Return("postgres", nil).Once()
		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql == "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(`[{"extversion": "2.8.0"}]`, nil).Once()

		mockUseCase.On("ExecuteStatement", mock.Anything, "timescale_db", mock.MatchedBy(func(sql string) bool {
			return sql != "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
		}), mock.Anything).Return(sampleHypertablesResult, nil).Once()

		// Create the context provider
		provider := mcp.NewTimescaleDBContextProvider()

		// Call the detection method
		contextInfo, err := provider.GetTimescaleDBContext(ctx, "timescale_db", mockUseCase)

		// Verify the result
		assert.NoError(t, err)
		assert.NotNil(t, contextInfo)
		assert.True(t, contextInfo.IsTimescaleDB)
		assert.Equal(t, "2.8.0", contextInfo.Version)
		assert.Len(t, contextInfo.Hypertables, 2)
		assert.Equal(t, "metrics", contextInfo.Hypertables[0].TableName)
		assert.Equal(t, "timestamp", contextInfo.Hypertables[0].TimeColumn)
		assert.Equal(t, "logs", contextInfo.Hypertables[1].TableName)
		assert.Equal(t, "log_time", contextInfo.Hypertables[1].TimeColumn)

		// Verify the mock expectations
		mockUseCase.AssertExpectations(t)
	})
}
