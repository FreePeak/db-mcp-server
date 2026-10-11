package mcp

import (
	"context"
	"testing"

	"github.com/FreePeak/cortex/pkg/server"
)

func TestToolFactory_DoesNotExposeFilesystemListing(t *testing.T) {
	if _, ok := NewToolTypeFactory().GetToolType("list"); ok {
		t.Fatal("filesystem list tool must not be part of the MCP tool surface")
	}
}

func TestRegisterAllTools_RejectsEmptyDatabaseConfiguration(t *testing.T) {
	useCase := new(MockDatabaseUseCase)
	useCase.On("ListDatabases").Return([]string{})
	registry := NewToolRegistry(server.NewMCPServer("test", "1.0.0", nil), false)

	if err := registry.RegisterAllTools(context.Background(), useCase); err == nil {
		t.Fatal("expected empty database configuration to fail tool registration")
	}
}
