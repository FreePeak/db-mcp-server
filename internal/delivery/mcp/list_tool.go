package mcp

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/tools"
)

// ListDirectoryTool handles listing files and directories
type ListDirectoryTool struct {
	BaseToolType
}

// NewListDirectoryTool creates a new list directory tool type
func NewListDirectoryTool() *ListDirectoryTool {
	return &ListDirectoryTool{
		BaseToolType: BaseToolType{
			name:        "list",
			description: "List files and directories in a given path",
		},
	}
}

// CreateTool creates a list directory tool
func (t *ListDirectoryTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("path",
			tools.Description("Absolute path to list"),
			tools.Required(),
		),
	)
}

// HandleRequest handles list directory tool requests
func (t *ListDirectoryTool) HandleRequest(_ context.Context, request server.ToolCallRequest, _ string, _ UseCaseProvider) (interface{}, error) {
	path, ok := request.Parameters["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Verify path exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", path)
		}
		return nil, fmt.Errorf("error accessing path: %w", err)
	}

	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Sort entries: directories first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contents of %s:\n\n", path))

	for _, entry := range entries {
		prefix := "[FILE]"
		if entry.IsDir() {
			prefix = "[DIR] "
		}

		info, err := entry.Info()
		size := "unknown"
		if err == nil {
			if !entry.IsDir() {
				size = fmt.Sprintf("%d bytes", info.Size())
			} else {
				size = "-"
			}
		}

		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", prefix, entry.Name(), size))
	}

	if len(entries) == 0 {
		sb.WriteString("(empty directory)")
	}

	return createTextResponse(sb.String()), nil
}
