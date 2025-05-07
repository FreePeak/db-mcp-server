package timescale

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// HypertableMetadata represents metadata about a TimescaleDB hypertable
type HypertableMetadata struct {
	TableName         string
	SchemaName        string
	Owner             string
	NumDimensions     int
	TimeDimension     string
	TimeDimensionType string
	SpaceDimensions   []string
	ChunkTimeInterval string
	Compression       bool
	RetentionPolicy   bool
	TotalSize         string
	TotalRows         int64
	Chunks            int
}

// ColumnMetadata represents metadata about a column
type ColumnMetadata struct {
	Name         string
	Type         string
	Nullable     bool
	IsPrimaryKey bool
	IsIndexed    bool
	Description  string
}

// ContinuousAggregateMetadata represents metadata about a continuous aggregate
type ContinuousAggregateMetadata struct {
	ViewName           string
	ViewSchema         string
	MaterializedOnly   bool
	RefreshInterval    string
	RefreshLag         string
	RefreshStartOffset string
	RefreshEndOffset   string
	HypertableName     string
	HypertableSchema   string
	ViewDefinition     string
}

// GetHypertableMetadata returns detailed metadata about a hypertable
func (t *DB) GetHypertableMetadata(ctx context.Context, tableName string) (*HypertableMetadata, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Query to get basic hypertable information
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

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get hypertable metadata: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", tableName)
	}

	row := rows[0]
	metadata := &HypertableMetadata{
		TableName:         fmt.Sprintf("%v", row["table_name"]),
		SchemaName:        fmt.Sprintf("%v", row["schema_name"]),
		Owner:             fmt.Sprintf("%v", row["owner"]),
		TimeDimension:     fmt.Sprintf("%v", row["time_dimension"]),
		TimeDimensionType: fmt.Sprintf("%v", row["time_dimension_type"]),
		ChunkTimeInterval: fmt.Sprintf("%v", row["chunk_time_interval"]),
		TotalSize:         fmt.Sprintf("%v", row["total_size"]),
	}

	// Convert numeric fields
	if numDimensions, ok := row["num_dimensions"].(int64); ok {
		metadata.NumDimensions = int(numDimensions)
	} else if numDimensions, ok := row["num_dimensions"].(int); ok {
		metadata.NumDimensions = numDimensions
	}

	if chunks, ok := row["chunks"].(int64); ok {
		metadata.Chunks = int(chunks)
	} else if chunks, ok := row["chunks"].(int); ok {
		metadata.Chunks = chunks
	}

	if rows, ok := row["total_rows"].(int64); ok {
		metadata.TotalRows = rows
	} else if rows, ok := row["total_rows"].(int); ok {
		metadata.TotalRows = int64(rows)
	} else if rowsStr, ok := row["total_rows"].(string); ok {
		if rows, err := strconv.ParseInt(rowsStr, 10, 64); err == nil {
			metadata.TotalRows = rows
		}
	}

	// Handle boolean fields
	if compression, ok := row["compression_enabled"].(bool); ok {
		metadata.Compression = compression
	} else if compressionStr, ok := row["compression_enabled"].(string); ok {
		metadata.Compression = compressionStr == "t" || compressionStr == "true" || compressionStr == "1"
	}

	// Get space dimensions if there are more than one dimension
	if metadata.NumDimensions > 1 {
		spaceDimQuery := fmt.Sprintf(`
			SELECT column_name
			FROM timescaledb_information.dimensions
			WHERE hypertable_name = '%s' AND dimension_number > 1
			ORDER BY dimension_number
		`, tableName)

		spaceResult, err := t.ExecuteSQLWithoutParams(ctx, spaceDimQuery)
		if err == nil {
			spaceDimRows, ok := spaceResult.([]map[string]interface{})
			if ok {
				for _, dimRow := range spaceDimRows {
					if colName, ok := dimRow["column_name"]; ok && colName != nil {
						metadata.SpaceDimensions = append(metadata.SpaceDimensions, fmt.Sprintf("%v", colName))
					}
				}
			}
		}
	}

	// Check if a retention policy exists
	retentionQuery := fmt.Sprintf(`
		SELECT COUNT(*) > 0 as has_retention
		FROM timescaledb_information.jobs
		WHERE hypertable_name = '%s' AND proc_name = 'policy_retention'
	`, tableName)

	retentionResult, err := t.ExecuteSQLWithoutParams(ctx, retentionQuery)
	if err == nil {
		retentionRows, ok := retentionResult.([]map[string]interface{})
		if ok && len(retentionRows) > 0 {
			if hasRetention, ok := retentionRows[0]["has_retention"].(bool); ok {
				metadata.RetentionPolicy = hasRetention
			}
		}
	}

	return metadata, nil
}

// GetTableColumns returns metadata about columns in a table
func (t *DB) GetTableColumns(ctx context.Context, tableName string) ([]ColumnMetadata, error) {
	query := fmt.Sprintf(`
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
				AND NOT i.indisprimary
				AND a.attname = c.column_name
			) as is_indexed,
			col_description(format('%%I.%%I', c.table_schema, c.table_name)::regclass::oid, 
							ordinal_position) as description
		FROM information_schema.columns c
		WHERE c.table_name = '%s'
		ORDER BY c.ordinal_position
	`, tableName)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	var columns []ColumnMetadata
	for _, row := range rows {
		col := ColumnMetadata{
			Name: fmt.Sprintf("%v", row["column_name"]),
			Type: fmt.Sprintf("%v", row["data_type"]),
		}

		// Handle boolean fields
		if nullable, ok := row["is_nullable"].(bool); ok {
			col.Nullable = nullable
		}
		if isPK, ok := row["is_primary_key"].(bool); ok {
			col.IsPrimaryKey = isPK
		}
		if isIndexed, ok := row["is_indexed"].(bool); ok {
			col.IsIndexed = isIndexed
		}

		// Handle description which might be null
		if desc, ok := row["description"]; ok && desc != nil {
			col.Description = fmt.Sprintf("%v", desc)
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// ListContinuousAggregates lists all continuous aggregates
func (t *DB) ListContinuousAggregates(ctx context.Context) ([]ContinuousAggregateMetadata, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	query := `
		SELECT 
			view_name,
			view_schema,
			materialized_only,
			refresh_lag,
			refresh_interval,
			hypertable_name,
			hypertable_schema
		FROM timescaledb_information.continuous_aggregates
	`

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list continuous aggregates: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	var aggregates []ContinuousAggregateMetadata
	for _, row := range rows {
		agg := ContinuousAggregateMetadata{
			ViewName:         fmt.Sprintf("%v", row["view_name"]),
			ViewSchema:       fmt.Sprintf("%v", row["view_schema"]),
			HypertableName:   fmt.Sprintf("%v", row["hypertable_name"]),
			HypertableSchema: fmt.Sprintf("%v", row["hypertable_schema"]),
		}

		// Handle boolean fields
		if materializedOnly, ok := row["materialized_only"].(bool); ok {
			agg.MaterializedOnly = materializedOnly
		}

		// Handle nullable fields
		if refreshLag, ok := row["refresh_lag"]; ok && refreshLag != nil {
			agg.RefreshLag = fmt.Sprintf("%v", refreshLag)
		}
		if refreshInterval, ok := row["refresh_interval"]; ok && refreshInterval != nil {
			agg.RefreshInterval = fmt.Sprintf("%v", refreshInterval)
		}

		// Get view definition
		definitionQuery := fmt.Sprintf(`
			SELECT pg_get_viewdef(format('%%I.%%I', '%s', '%s')::regclass, true) as view_definition
		`, agg.ViewSchema, agg.ViewName)

		defResult, err := t.ExecuteSQLWithoutParams(ctx, definitionQuery)
		if err == nil {
			defRows, ok := defResult.([]map[string]interface{})
			if ok && len(defRows) > 0 {
				if def, ok := defRows[0]["view_definition"]; ok && def != nil {
					agg.ViewDefinition = fmt.Sprintf("%v", def)
				}
			}
		}

		aggregates = append(aggregates, agg)
	}

	return aggregates, nil
}

// GetContinuousAggregate gets metadata about a specific continuous aggregate
func (t *DB) GetContinuousAggregate(ctx context.Context, viewName string) (*ContinuousAggregateMetadata, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf(`
		SELECT 
			view_name,
			view_schema,
			materialized_only,
			refresh_lag,
			refresh_interval,
			hypertable_name,
			hypertable_schema
		FROM timescaledb_information.continuous_aggregates
		WHERE view_name = '%s'
	`, viewName)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get continuous aggregate: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("continuous aggregate '%s' not found", viewName)
	}

	row := rows[0]
	agg := &ContinuousAggregateMetadata{
		ViewName:         fmt.Sprintf("%v", row["view_name"]),
		ViewSchema:       fmt.Sprintf("%v", row["view_schema"]),
		HypertableName:   fmt.Sprintf("%v", row["hypertable_name"]),
		HypertableSchema: fmt.Sprintf("%v", row["hypertable_schema"]),
	}

	// Handle boolean fields
	if materializedOnly, ok := row["materialized_only"].(bool); ok {
		agg.MaterializedOnly = materializedOnly
	}

	// Handle nullable fields
	if refreshLag, ok := row["refresh_lag"]; ok && refreshLag != nil {
		agg.RefreshLag = fmt.Sprintf("%v", refreshLag)
	}
	if refreshInterval, ok := row["refresh_interval"]; ok && refreshInterval != nil {
		agg.RefreshInterval = fmt.Sprintf("%v", refreshInterval)
	}

	// Get view definition
	definitionQuery := fmt.Sprintf(`
		SELECT pg_get_viewdef(format('%%I.%%I', '%s', '%s')::regclass, true) as view_definition
	`, agg.ViewSchema, agg.ViewName)

	defResult, err := t.ExecuteSQLWithoutParams(ctx, definitionQuery)
	if err == nil {
		defRows, ok := defResult.([]map[string]interface{})
		if ok && len(defRows) > 0 {
			if def, ok := defRows[0]["view_definition"]; ok && def != nil {
				agg.ViewDefinition = fmt.Sprintf("%v", def)
			}
		}
	}

	return agg, nil
}

// GetDatabaseSize gets size information about the database
func (t *DB) GetDatabaseSize(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT 
			pg_size_pretty(pg_database_size(current_database())) as database_size,
			current_database() as database_name,
			(
				SELECT pg_size_pretty(sum(pg_total_relation_size(format('%I.%I', h.schema_name, h.table_name))))
				FROM timescaledb_information.hypertables h
			) as hypertables_size,
			(
				SELECT count(*)
				FROM timescaledb_information.hypertables
			) as hypertables_count
	`

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("failed to get database size information")
	}

	info := make(map[string]string)
	for k, v := range rows[0] {
		if v != nil {
			info[k] = fmt.Sprintf("%v", v)
		}
	}

	return info, nil
}

// DetectTimescaleDBVersion checks if TimescaleDB is installed and returns its version
func (t *DB) DetectTimescaleDBVersion(ctx context.Context) (string, error) {
	query := "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"
	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to check TimescaleDB version: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return "", fmt.Errorf("TimescaleDB extension not installed")
	}

	version := rows[0]["extversion"]
	if version == nil {
		return "", fmt.Errorf("unable to determine TimescaleDB version")
	}

	return fmt.Sprintf("%v", version), nil
}

// GenerateHypertableSchema generates CREATE TABLE and CREATE HYPERTABLE statements for a hypertable
func (t *DB) GenerateHypertableSchema(ctx context.Context, tableName string) (string, error) {
	if !t.isTimescaleDB {
		return "", fmt.Errorf("TimescaleDB extension not available")
	}

	// Get table columns and constraints
	columnsQuery := fmt.Sprintf(`
		SELECT 
			'CREATE TABLE ' || quote_ident('%s') || ' (' ||
			string_agg(
				quote_ident(column_name) || ' ' || 
				data_type || 
				CASE 
					WHEN character_maximum_length IS NOT NULL THEN '(' || character_maximum_length || ')'
					WHEN numeric_precision IS NOT NULL AND numeric_scale IS NOT NULL THEN '(' || numeric_precision || ',' || numeric_scale || ')'
					ELSE ''
				END ||
				CASE WHEN is_nullable = 'NO' THEN ' NOT NULL' ELSE '' END,
				', '
			) ||
			CASE 
				WHEN (
					SELECT count(*) > 0 
					FROM information_schema.table_constraints tc
					WHERE tc.table_name = '%s' AND tc.constraint_type = 'PRIMARY KEY'
				) THEN 
					', ' || (
						SELECT 'PRIMARY KEY (' || string_agg(quote_ident(kcu.column_name), ', ') || ')'
						FROM information_schema.table_constraints tc
						JOIN information_schema.key_column_usage kcu ON 
							kcu.constraint_name = tc.constraint_name AND
							kcu.table_schema = tc.table_schema AND
							kcu.table_name = tc.table_name
						WHERE tc.table_name = '%s' AND tc.constraint_type = 'PRIMARY KEY'
					)
				ELSE ''
			END ||
			');' as create_table_stmt
		FROM information_schema.columns
		WHERE table_name = '%s'
		GROUP BY table_name
	`, tableName, tableName, tableName, tableName)

	columnsResult, err := t.ExecuteSQLWithoutParams(ctx, columnsQuery)
	if err != nil {
		return "", fmt.Errorf("failed to generate schema: %w", err)
	}

	columnsRows, ok := columnsResult.([]map[string]interface{})
	if !ok || len(columnsRows) == 0 {
		return "", fmt.Errorf("failed to generate schema for table '%s'", tableName)
	}

	createTableStmt := fmt.Sprintf("%v", columnsRows[0]["create_table_stmt"])

	// Get hypertable metadata
	metadata, err := t.GetHypertableMetadata(ctx, tableName)
	if err != nil {
		return createTableStmt, nil // Return just the CREATE TABLE statement if it's not a hypertable
	}

	// Generate CREATE HYPERTABLE statement
	var createHypertableStmt strings.Builder
	createHypertableStmt.WriteString(fmt.Sprintf("SELECT create_hypertable('%s', '%s'",
		tableName, metadata.TimeDimension))

	if metadata.ChunkTimeInterval != "" {
		createHypertableStmt.WriteString(fmt.Sprintf(", chunk_time_interval => INTERVAL '%s'",
			metadata.ChunkTimeInterval))
	}

	if len(metadata.SpaceDimensions) > 0 {
		createHypertableStmt.WriteString(fmt.Sprintf(", partitioning_column => '%s'",
			metadata.SpaceDimensions[0]))
	}

	createHypertableStmt.WriteString(");")

	// Combine statements
	result := createTableStmt + "\n\n" + createHypertableStmt.String()

	// Add compression statement if enabled
	if metadata.Compression {
		compressionSettings, err := t.GetCompressionSettings(ctx, tableName)
		if err == nil && compressionSettings.CompressionEnabled {
			compressionStmt := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = true);", tableName)
			result += "\n\n" + compressionStmt

			// Add compression policy if exists
			if compressionSettings.CompressionInterval != "" {
				policyStmt := fmt.Sprintf("SELECT add_compression_policy('%s', INTERVAL '%s'",
					tableName, compressionSettings.CompressionInterval)

				if compressionSettings.SegmentBy != "" {
					policyStmt += fmt.Sprintf(", segmentby => '%s'", compressionSettings.SegmentBy)
				}

				if compressionSettings.OrderBy != "" {
					policyStmt += fmt.Sprintf(", orderby => '%s'", compressionSettings.OrderBy)
				}

				policyStmt += ");"
				result += "\n" + policyStmt
			}
		}
	}

	// Add retention policy if enabled
	if metadata.RetentionPolicy {
		retentionSettings, err := t.GetRetentionSettings(ctx, tableName)
		if err == nil && retentionSettings.RetentionEnabled && retentionSettings.RetentionInterval != "" {
			retentionStmt := fmt.Sprintf("SELECT add_retention_policy('%s', INTERVAL '%s');",
				tableName, retentionSettings.RetentionInterval)
			result += "\n\n" + retentionStmt
		}
	}

	return result, nil
}
