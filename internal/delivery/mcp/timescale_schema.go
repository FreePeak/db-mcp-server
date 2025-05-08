package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// UseCaseProvider interface defined in the package

// HypertableSchemaInfo represents schema information for a TimescaleDB hypertable
type HypertableSchemaInfo struct {
	TableName          string                 `json:"tableName"`
	SchemaName         string                 `json:"schemaName"`
	TimeColumn         string                 `json:"timeColumn"`
	ChunkTimeInterval  string                 `json:"chunkTimeInterval"`
	Size               string                 `json:"size"`
	ChunkCount         int                    `json:"chunkCount"`
	RowCount           int64                  `json:"rowCount"`
	SpacePartitioning  []string               `json:"spacePartitioning,omitempty"`
	CompressionEnabled bool                   `json:"compressionEnabled"`
	CompressionConfig  CompressionConfig      `json:"compressionConfig,omitempty"`
	RetentionEnabled   bool                   `json:"retentionEnabled"`
	RetentionInterval  string                 `json:"retentionInterval,omitempty"`
	Columns            []HypertableColumnInfo `json:"columns"`
}

// HypertableColumnInfo represents column information for a hypertable
type HypertableColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	PrimaryKey  bool   `json:"primaryKey"`
	Indexed     bool   `json:"indexed"`
	Description string `json:"description,omitempty"`
}

// CompressionConfig represents compression configuration for a hypertable
type CompressionConfig struct {
	SegmentBy string `json:"segmentBy,omitempty"`
	OrderBy   string `json:"orderBy,omitempty"`
	Interval  string `json:"interval,omitempty"`
}

// HypertableSchemaProvider provides schema information for hypertables
type HypertableSchemaProvider struct {
	// We use the TimescaleDBContextProvider from timescale_context.go
	contextProvider *TimescaleDBContextProvider
}

// NewHypertableSchemaProvider creates a new hypertable schema provider
func NewHypertableSchemaProvider() *HypertableSchemaProvider {
	return &HypertableSchemaProvider{
		contextProvider: NewTimescaleDBContextProvider(),
	}
}

// GetHypertableSchema gets schema information for a specific hypertable
func (p *HypertableSchemaProvider) GetHypertableSchema(
	ctx context.Context,
	dbID string,
	tableName string,
	useCase UseCaseProvider,
) (*HypertableSchemaInfo, error) {
	// First check if TimescaleDB is available
	tsdbContext, err := p.contextProvider.DetectTimescaleDB(ctx, dbID, useCase)
	if err != nil {
		return nil, fmt.Errorf("failed to detect TimescaleDB: %w", err)
	}

	if !tsdbContext.IsTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB is not available in the database %s", dbID)
	}

	// Get hypertable metadata
	query := fmt.Sprintf(`
		SELECT 
			h.table_name,
			h.schema_name,
			t.tableowner as owner,
			h.num_dimensions,
			dc.column_name as time_dimension,
			dc.column_type as time_dimension_type,
			dc.time_interval as chunk_time_interval,
			h.compression_enabled,
			pg_size_pretty(pg_total_relation_size(format('%%I.%%I', h.schema_name, h.table_name))) as total_size,
			(SELECT count(*) FROM timescaledb_information.chunks WHERE hypertable_name = h.table_name) as chunks,
			(SELECT count(*) FROM %s.%s) as total_rows
		FROM timescaledb_information.hypertables h
		JOIN pg_tables t ON h.table_name = t.tablename AND h.schema_name = t.schemaname
		JOIN timescaledb_information.dimensions dc ON h.hypertable_name = dc.hypertable_name
		WHERE h.table_name = '%s' AND dc.dimension_number = 1
	`, tableName, tableName, tableName)

	metadataResult, err := useCase.ExecuteStatement(ctx, dbID, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get hypertable metadata: %w", err)
	}

	// Parse the result to determine if the table is a hypertable
	var metadata []map[string]interface{}
	if err := json.Unmarshal([]byte(metadataResult), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata result: %w", err)
	}

	// If no results, the table is not a hypertable
	if len(metadata) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", tableName)
	}

	// Create schema info from metadata
	schemaInfo := &HypertableSchemaInfo{
		TableName: tableName,
		Columns:   []HypertableColumnInfo{},
	}

	// Extract metadata fields
	row := metadata[0]

	if schemaName, ok := row["schema_name"].(string); ok {
		schemaInfo.SchemaName = schemaName
	}

	if timeDimension, ok := row["time_dimension"].(string); ok {
		schemaInfo.TimeColumn = timeDimension
	}

	if chunkInterval, ok := row["chunk_time_interval"].(string); ok {
		schemaInfo.ChunkTimeInterval = chunkInterval
	}

	if size, ok := row["total_size"].(string); ok {
		schemaInfo.Size = size
	}

	// Convert numeric fields
	if chunks, ok := row["chunks"].(float64); ok {
		schemaInfo.ChunkCount = int(chunks)
	} else if chunks, ok := row["chunks"].(int); ok {
		schemaInfo.ChunkCount = chunks
	} else if chunksStr, ok := row["chunks"].(string); ok {
		if chunks, err := strconv.Atoi(chunksStr); err == nil {
			schemaInfo.ChunkCount = chunks
		}
	}

	if rows, ok := row["total_rows"].(float64); ok {
		schemaInfo.RowCount = int64(rows)
	} else if rows, ok := row["total_rows"].(int64); ok {
		schemaInfo.RowCount = rows
	} else if rowsStr, ok := row["total_rows"].(string); ok {
		if rows, err := strconv.ParseInt(rowsStr, 10, 64); err == nil {
			schemaInfo.RowCount = rows
		}
	}

	// Handle boolean fields
	if compression, ok := row["compression_enabled"].(bool); ok {
		schemaInfo.CompressionEnabled = compression
	} else if compressionStr, ok := row["compression_enabled"].(string); ok {
		schemaInfo.CompressionEnabled = compressionStr == "t" || compressionStr == "true" || compressionStr == "1"
	}

	// Get compression settings if compression is enabled
	if schemaInfo.CompressionEnabled {
		compressionQuery := fmt.Sprintf(`
			SELECT segmentby, orderby, compression_interval
			FROM (
				SELECT 
					cs.segmentby, 
					cs.orderby,
					(SELECT schedule_interval FROM timescaledb_information.job_stats js 
					JOIN timescaledb_information.jobs j ON js.job_id = j.job_id 
					WHERE j.hypertable_name = '%s' AND j.proc_name = 'policy_compression'
					LIMIT 1) as compression_interval
				FROM timescaledb_information.compression_settings cs
				WHERE cs.hypertable_name = '%s'
			) t
		`, tableName, tableName)

		compressionResult, err := useCase.ExecuteStatement(ctx, dbID, compressionQuery, nil)
		if err == nil {
			var compressionSettings []map[string]interface{}
			if err := json.Unmarshal([]byte(compressionResult), &compressionSettings); err == nil && len(compressionSettings) > 0 {
				settings := compressionSettings[0]

				if segmentBy, ok := settings["segmentby"].(string); ok {
					schemaInfo.CompressionConfig.SegmentBy = segmentBy
				}

				if orderBy, ok := settings["orderby"].(string); ok {
					schemaInfo.CompressionConfig.OrderBy = orderBy
				}

				if interval, ok := settings["compression_interval"].(string); ok {
					schemaInfo.CompressionConfig.Interval = interval
				}
			}
		}
	}

	// Get retention settings
	retentionQuery := fmt.Sprintf(`
		SELECT 
			hypertable_name,
			schedule_interval as retention_interval,
			TRUE as retention_enabled
		FROM 
			timescaledb_information.jobs j
		JOIN 
			timescaledb_information.job_stats js ON j.job_id = js.job_id
		WHERE 
			j.hypertable_name = '%s' AND j.proc_name = 'policy_retention'
	`, tableName)

	retentionResult, err := useCase.ExecuteStatement(ctx, dbID, retentionQuery, nil)
	if err == nil {
		var retentionSettings []map[string]interface{}
		if err := json.Unmarshal([]byte(retentionResult), &retentionSettings); err == nil && len(retentionSettings) > 0 {
			settings := retentionSettings[0]

			schemaInfo.RetentionEnabled = true

			if interval, ok := settings["retention_interval"].(string); ok {
				schemaInfo.RetentionInterval = interval
			}
		}
	}

	// Get column information
	columnsQuery := fmt.Sprintf(`
		SELECT 
			c.column_name, 
			c.data_type,
			c.is_nullable = 'YES' as is_nullable,
			(
				SELECT COUNT(*) > 0
				FROM pg_index i
				JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
				WHERE i.indrelid = format('%%I.%%I', c.table_schema, c.table_name)::regclass
				AND i.indisprimary
				AND a.attname = c.column_name
			) as is_primary_key,
			(
				SELECT COUNT(*) > 0
				FROM pg_index i
				JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
				WHERE i.indrelid = format('%%I.%%I', c.table_schema, c.table_name)::regclass
				AND a.attname = c.column_name
			) as is_indexed,
			col_description(format('%%I.%%I', c.table_schema, c.table_name)::regclass::oid, 
							ordinal_position) as description
		FROM information_schema.columns c
		WHERE c.table_name = '%s'
		ORDER BY c.ordinal_position
	`, tableName)

	columnsResult, err := useCase.ExecuteStatement(ctx, dbID, columnsQuery, nil)
	if err == nil {
		var columns []map[string]interface{}
		if err := json.Unmarshal([]byte(columnsResult), &columns); err == nil {
			for _, column := range columns {
				columnInfo := HypertableColumnInfo{}

				if name, ok := column["column_name"].(string); ok {
					columnInfo.Name = name
				}

				if dataType, ok := column["data_type"].(string); ok {
					columnInfo.Type = dataType
				}

				if nullable, ok := column["is_nullable"].(bool); ok {
					columnInfo.Nullable = nullable
				}

				if primaryKey, ok := column["is_primary_key"].(bool); ok {
					columnInfo.PrimaryKey = primaryKey
				}

				if indexed, ok := column["is_indexed"].(bool); ok {
					columnInfo.Indexed = indexed
				}

				if description, ok := column["description"].(string); ok {
					columnInfo.Description = description
				}

				schemaInfo.Columns = append(schemaInfo.Columns, columnInfo)
			}
		}
	}

	return schemaInfo, nil
}
