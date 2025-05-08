package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Import and use the UseCaseProvider interface from the timescale_tool.go file
// UseCaseProvider is defined as:
// type UseCaseProvider interface {
//   ExecuteQuery(ctx context.Context, dbID, query string, params []interface{}) (string, error)
//   ExecuteStatement(ctx context.Context, dbID, statement string, params []interface{}) (string, error)
//   ExecuteTransaction(ctx context.Context, dbID, action string, txID string, statement string, params []interface{}, readOnly bool) (string, map[string]interface{}, error)
//   GetDatabaseInfo(dbID string) (map[string]interface{}, error)
//   ListDatabases() []string
//   GetDatabaseType(dbID string) (string, error)
// }

// TimescaleDBContextInfo represents information about TimescaleDB for editor context
type TimescaleDBContextInfo struct {
	IsTimescaleDB bool                        `json:"isTimescaleDB"`
	Version       string                      `json:"version,omitempty"`
	Hypertables   []TimescaleDBHypertableInfo `json:"hypertables,omitempty"`
}

// TimescaleDBHypertableInfo contains information about a hypertable
type TimescaleDBHypertableInfo struct {
	TableName     string `json:"tableName"`
	TimeColumn    string `json:"timeColumn"`
	ChunkInterval string `json:"chunkInterval"`
}

// TimescaleDBContextProvider provides TimescaleDB information for editor context
type TimescaleDBContextProvider struct{}

// NewTimescaleDBContextProvider creates a new TimescaleDB context provider
func NewTimescaleDBContextProvider() *TimescaleDBContextProvider {
	return &TimescaleDBContextProvider{}
}

// DetectTimescaleDB detects if TimescaleDB is installed in the given database
func (p *TimescaleDBContextProvider) DetectTimescaleDB(ctx context.Context, dbID string, useCase UseCaseProvider) (*TimescaleDBContextInfo, error) {
	// Check database type first
	dbType, err := useCase.GetDatabaseType(dbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get database type: %w", err)
	}

	// TimescaleDB is a PostgreSQL extension, so we only check PostgreSQL databases
	if !strings.Contains(strings.ToLower(dbType), "postgres") {
		// Return a context info object with isTimescaleDB = false
		return &TimescaleDBContextInfo{
			IsTimescaleDB: false,
		}, nil
	}

	// Check for TimescaleDB extension
	query := "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
	result, err := useCase.ExecuteStatement(ctx, dbID, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check for TimescaleDB extension: %w", err)
	}

	// Parse the result to determine if TimescaleDB is available
	var versions []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &versions); err != nil {
		return nil, fmt.Errorf("failed to parse extension result: %w", err)
	}

	// If no results, TimescaleDB is not installed
	if len(versions) == 0 {
		return &TimescaleDBContextInfo{
			IsTimescaleDB: false,
		}, nil
	}

	// Extract version information
	version := ""
	if extVersion, ok := versions[0]["extversion"]; ok && extVersion != nil {
		version = fmt.Sprintf("%v", extVersion)
	}

	// Create and return context info
	return &TimescaleDBContextInfo{
		IsTimescaleDB: true,
		Version:       version,
	}, nil
}

// GetTimescaleDBContext gets comprehensive TimescaleDB context information
func (p *TimescaleDBContextProvider) GetTimescaleDBContext(ctx context.Context, dbID string, useCase UseCaseProvider) (*TimescaleDBContextInfo, error) {
	// First, detect if TimescaleDB is available
	contextInfo, err := p.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, err
	}

	// If not TimescaleDB, return basic info
	if !contextInfo.IsTimescaleDB {
		return contextInfo, nil
	}

	// Get information about hypertables
	query := `
		SELECT 
			h.table_name,
			d.column_name as time_column,
			d.time_interval as chunk_interval
		FROM 
			_timescaledb_catalog.hypertable h
		JOIN 
			_timescaledb_catalog.dimension d ON h.id = d.hypertable_id
		WHERE 
			d.column_type = 'TIMESTAMP' OR d.column_type = 'TIMESTAMPTZ'
		ORDER BY 
			h.table_name
	`

	result, err := useCase.ExecuteStatement(ctx, dbID, query, nil)
	if err != nil {
		// Don't fail the whole context if just hypertable info fails
		return contextInfo, nil
	}

	// Parse the result
	var hypertables []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &hypertables); err != nil {
		return contextInfo, nil
	}

	// Process hypertable information
	for _, h := range hypertables {
		hypertableInfo := TimescaleDBHypertableInfo{}

		if tableName, ok := h["table_name"]; ok && tableName != nil {
			hypertableInfo.TableName = fmt.Sprintf("%v", tableName)
		}

		if timeColumn, ok := h["time_column"]; ok && timeColumn != nil {
			hypertableInfo.TimeColumn = fmt.Sprintf("%v", timeColumn)
		}

		if chunkInterval, ok := h["chunk_interval"]; ok && chunkInterval != nil {
			hypertableInfo.ChunkInterval = fmt.Sprintf("%v", chunkInterval)
		}

		// Only add if we have a valid table name
		if hypertableInfo.TableName != "" {
			contextInfo.Hypertables = append(contextInfo.Hypertables, hypertableInfo)
		}
	}

	return contextInfo, nil
}
