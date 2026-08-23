package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreePeak/cortex/pkg/server"
	"github.com/FreePeak/cortex/pkg/tools"
)

// GenerateSchemaTool renders the live schema as application code (Go
// structs, TypeScript interfaces) so agents can bind types to real columns.
type GenerateSchemaTool struct {
	BaseToolType
}

// NewGenerateSchemaTool creates a new generate-schema tool type
func NewGenerateSchemaTool() *GenerateSchemaTool {
	return &GenerateSchemaTool{
		BaseToolType: BaseToolType{
			name:        "generate_schema",
			description: "Generate SQL or code from database schema",
		},
	}
}

const generateSchemaFormatHelp = `Target language: "go" (structs with db tags) or "typescript" (interfaces)`

// CreateTool creates a generate-schema tool for a specific database
func (t *GenerateSchemaTool) CreateTool(name string, dbID string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetDescription(dbID)),
		tools.WithString("format",
			tools.Description(generateSchemaFormatHelp),
			tools.Required(),
		),
	)
}

// CreateUnifiedTool creates a unified generate-schema tool with a database parameter
func (t *GenerateSchemaTool) CreateUnifiedTool(name string, dbList []string) interface{} {
	return tools.NewTool(
		name,
		tools.WithDescription(t.GetUnifiedDescription(dbList)),
		tools.WithString("database",
			tools.Description(fmt.Sprintf("Database ID to use. Available: %s", strings.Join(dbList, ", "))),
			tools.Required(),
		),
		tools.WithString("format",
			tools.Description(generateSchemaFormatHelp),
			tools.Required(),
		),
	)
}

// HandleRequest handles generate-schema tool requests
func (t *GenerateSchemaTool) HandleRequest(ctx context.Context, request server.ToolCallRequest, dbID string, useCase UseCaseProvider) (interface{}, error) {
	if dbID == "" {
		dbID = extractDatabaseIDFromName(request.Name)
	}
	target, _ := request.Parameters["format"].(string) //nolint:errcheck // absent means empty target
	if target == "" {
		target, _ = request.Parameters["target"].(string) //nolint:errcheck // alias accepted
	}
	code, err := useCase.GenerateSchemaCode(ctx, dbID, target)
	if err != nil {
		return nil, err
	}
	return createTextResponse(code), nil
}
