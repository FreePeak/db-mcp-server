package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUseCaseProvider for testing
type MockUseCaseProvider struct {
	mock.Mock
}

func (m *MockUseCaseProvider) ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, query, params)
	return args.String(0), args.Error(1)
}

func (m *MockUseCaseProvider) ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error) {
	args := m.Called(ctx, dbID, statement, params)
	return args.String(0), args.Error(1)
}

func (m *MockUseCaseProvider) ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error) {
	args := m.Called(ctx, dbID, action, txID, statement, params, readOnly)
	return args.String(0), args.Get(1).(map[string]interface{}), args.Error(2)
}

func (m *MockUseCaseProvider) GetDatabaseInfo(dbID string) (map[string]interface{}, error) {
	args := m.Called(dbID)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockUseCaseProvider) ListDatabases() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockUseCaseProvider) GetDatabaseType(dbID string) (string, error) {
	args := m.Called(dbID)
	return args.String(0), args.Error(1)
}

func (m *MockUseCaseProvider) IsLazyLoading() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockUseCaseProvider) FilterTableNames(ctx context.Context, dbID, pattern string) ([]string, error) {
	args := m.Called(ctx, dbID, pattern)
	return args.Get(0).([]string), args.Error(1)
}

func TestListDirectoryTool(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "list_tool_test")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some files and directories
	assert.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content"), 0644))
	assert.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir"), 0755))

	tool := NewListDirectoryTool()
	assert.Equal(t, "list", tool.GetName())
	assert.Contains(t, tool.GetDescription(""), "List files and directories")

	// Test CreateTool
	toolDef := tool.CreateTool("list", "")
	assert.NotNil(t, toolDef)

	// Test HandleRequest - Success
	req := server.ToolCallRequest{
		Name: "list",
		Parameters: map[string]interface{}{
			"path": tempDir,
		},
	}

	mockUseCase := &MockUseCaseProvider{}
	resp, err := tool.HandleRequest(context.Background(), req, "", mockUseCase)
	assert.NoError(t, err)

	respMap, ok := resp.(map[string]interface{})
	assert.True(t, ok)
	content, ok := respMap["content"].([]map[string]interface{})
	assert.True(t, ok)
	assert.Len(t, content, 1)
	text, ok := content[0]["text"].(string)
	assert.True(t, ok)
	assert.Contains(t, text, "file1.txt")
	assert.Contains(t, text, "subdir")

	// Test HandleRequest - Non-existent path
	req.Parameters["path"] = filepath.Join(tempDir, "nonexistent")
	_, err = tool.HandleRequest(context.Background(), req, "", mockUseCase)
	assert.Error(t, err)
}
