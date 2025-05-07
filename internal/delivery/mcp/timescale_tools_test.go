package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

func TestTimescaleDBToolRegistration(t *testing.T) {
	// Create a new tool type factory
	factory := mcp.NewToolTypeFactory()

	// Verify that TimescaleDB tool type doesn't exist yet
	_, ok := factory.GetToolType("timescaledb")
	assert.False(t, ok, "TimescaleDB tool type should not exist yet")

	// We're just testing that a TimescaleDB tool can be created
	tool := mcp.NewTimescaleDBTool()
	assert.NotNil(t, tool, "Successfully created TimescaleDB tool type")
	assert.Equal(t, "timescaledb", tool.GetName())
}
