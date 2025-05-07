package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/FreePeak/db-mcp-server/internal/delivery/mcp"
)

func TestNewTimescaleDBTool(t *testing.T) {
	// Create a new TimescaleDB tool
	timescaleTool := mcp.NewTimescaleDBTool()

	// Test that the name is correct
	assert.Equal(t, "timescaledb", timescaleTool.GetName())

	// Test that the description is correct
	description := timescaleTool.GetDescription("test_db")
	assert.Contains(t, description, "TimescaleDB")
}
