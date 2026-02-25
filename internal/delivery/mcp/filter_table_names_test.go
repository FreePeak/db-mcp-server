package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFilterTableNamesTool_CreateTool(t *testing.T) {
	tool := NewFilterTableNamesTool()

	result := tool.CreateTool("filter_table_names_testdb", "testdb")
	assert.NotNil(t, result)
}

func TestFilterTableNamesTool_HandleRequest(t *testing.T) {
	tests := []struct {
		name        string
		dbID        string
		pattern     string
		mockTables  []string
		mockErr     error
		wantErr     bool
		errContains string
	}{
		{
			name:       "match found - single table",
			dbID:       "testdb",
			pattern:    "wp_",
			mockTables: []string{"wp_users"},
			wantErr:    false,
		},
		{
			name:       "match found - multiple tables",
			dbID:       "testdb",
			pattern:    "wp_",
			mockTables: []string{"wp_comments", "wp_posts", "wp_users"},
			wantErr:    false,
		},
		{
			name:       "no match found",
			dbID:       "testdb",
			pattern:    "xyz",
			mockTables: []string{},
			wantErr:    false,
		},
		{
			name:       "case insensitive match",
			dbID:       "testdb",
			pattern:    "WP_",
			mockTables: []string{"wp_users", "WP_Posts"},
			wantErr:    false,
		},
		{
			name:        "missing pattern",
			dbID:        "testdb",
			pattern:     "",
			wantErr:     true,
			errContains: "pattern parameter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUseCase := new(MockDatabaseUseCase)
			tool := NewFilterTableNamesTool()

			if tt.pattern != "" {
				mockUseCase.On("FilterTableNames", mock.Anything, tt.dbID, tt.pattern).
					Return(tt.mockTables, tt.mockErr)
			}

			request := server.ToolCallRequest{
				Name: "filter_table_names_testdb",
				Parameters: map[string]interface{}{
					"pattern": tt.pattern,
				},
			}

			result, err := tool.HandleRequest(context.Background(), request, tt.dbID, mockUseCase)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				resp, ok := result.(map[string]interface{})
				assert.True(t, ok)
				assert.Contains(t, resp, "content")

				content := resp["content"].([]map[string]interface{})
				assert.Len(t, content, 1)
				assert.Equal(t, "text", content[0]["type"])
				assert.NotEmpty(t, content[0]["text"])

				mockUseCase.AssertExpectations(t)
			}
		})
	}
}

func TestFilterTableNamesTool_HandleRequest_ExtractsDBIDFromName(t *testing.T) {
	mockUseCase := new(MockDatabaseUseCase)
	tool := NewFilterTableNamesTool()

	mockUseCase.On("FilterTableNames", mock.Anything, "mydb", "test").
		Return([]string{"test_table"}, nil)

	request := server.ToolCallRequest{
		Name: "filter_table_names_mydb",
		Parameters: map[string]interface{}{
			"pattern": "test",
		},
	}

	result, err := tool.HandleRequest(context.Background(), request, "", mockUseCase)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockUseCase.AssertExpectations(t)
}
