package timescale

import (
	"context"
	"fmt"
	"strings"
)

// HypertableConfig defines configuration for creating a hypertable
type HypertableConfig struct {
	TableName          string
	TimeColumn         string
	ChunkTimeInterval  string
	PartitioningColumn string
	CreateIfNotExists  bool
	SpacePartitions    int  // Number of space partitions (for multi-dimensional partitioning)
	IfNotExists        bool // If true, don't error if table is already a hypertable
	MigrateData        bool // If true, migrate existing data to chunks
}

// Hypertable represents a TimescaleDB hypertable
type Hypertable struct {
	TableName          string
	SchemaName         string
	TimeColumn         string
	SpaceColumn        string
	NumDimensions      int
	CompressionEnabled bool
	RetentionEnabled   bool
}

// CreateHypertable converts a regular PostgreSQL table to a TimescaleDB hypertable
func (t *TimescaleDB) CreateHypertable(ctx context.Context, config HypertableConfig) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Construct the create_hypertable call
	var queryBuilder strings.Builder
	queryBuilder.WriteString("SELECT create_hypertable(")

	// Table name and time column are required
	queryBuilder.WriteString(fmt.Sprintf("'%s', '%s'", config.TableName, config.TimeColumn))

	// Optional parameters
	if config.PartitioningColumn != "" {
		queryBuilder.WriteString(fmt.Sprintf(", partition_column => '%s'", config.PartitioningColumn))
	}

	if config.ChunkTimeInterval != "" {
		queryBuilder.WriteString(fmt.Sprintf(", chunk_time_interval => INTERVAL '%s'", config.ChunkTimeInterval))
	}

	if config.SpacePartitions > 0 {
		queryBuilder.WriteString(fmt.Sprintf(", number_partitions => %d", config.SpacePartitions))
	}

	if config.IfNotExists {
		queryBuilder.WriteString(", if_not_exists => TRUE")
	}

	if config.MigrateData {
		queryBuilder.WriteString(", migrate_data => TRUE")
	}

	queryBuilder.WriteString(")")

	// Execute the query
	_, err := t.ExecuteSQLWithoutParams(ctx, queryBuilder.String())
	if err != nil {
		return fmt.Errorf("failed to create hypertable: %w", err)
	}

	return nil
}

// AddDimension adds a new dimension (partitioning key) to a hypertable
func (t *TimescaleDB) AddDimension(ctx context.Context, tableName, columnName string, numPartitions int) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf("SELECT add_dimension('%s', '%s', number_partitions => %d)",
		tableName, columnName, numPartitions)

	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to add dimension: %w", err)
	}

	return nil
}

// ListHypertables returns a list of all hypertables in the database
func (t *TimescaleDB) ListHypertables(ctx context.Context) ([]Hypertable, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	query := `
		SELECT h.table_name, h.schema_name, d.column_name as time_column,
			count(d.id) as num_dimensions,
			(
				SELECT column_name FROM _timescaledb_catalog.dimension 
				WHERE hypertable_id = h.id AND column_type != 'TIMESTAMP' 
				AND column_type != 'TIMESTAMPTZ' 
				LIMIT 1
			) as space_column
		FROM _timescaledb_catalog.hypertable h
		JOIN _timescaledb_catalog.dimension d ON h.id = d.hypertable_id
		GROUP BY h.id, h.table_name, h.schema_name
	`

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list hypertables: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from database query")
	}

	var hypertables []Hypertable
	for _, row := range rows {
		ht := Hypertable{
			TableName:  fmt.Sprintf("%v", row["table_name"]),
			SchemaName: fmt.Sprintf("%v", row["schema_name"]),
			TimeColumn: fmt.Sprintf("%v", row["time_column"]),
		}

		// Handle nullable columns
		if row["space_column"] != nil {
			ht.SpaceColumn = fmt.Sprintf("%v", row["space_column"])
		}

		// Convert numeric dimensions
		if dimensions, ok := row["num_dimensions"].(int64); ok {
			ht.NumDimensions = int(dimensions)
		} else if dimensions, ok := row["num_dimensions"].(int); ok {
			ht.NumDimensions = dimensions
		}

		// Check if compression is enabled
		compQuery := fmt.Sprintf(
			"SELECT count(*) > 0 as is_compressed FROM timescaledb_information.compression_settings WHERE hypertable_name = '%s'",
			ht.TableName,
		)
		compResult, err := t.ExecuteSQLWithoutParams(ctx, compQuery)
		if err == nil {
			if compRows, ok := compResult.([]map[string]interface{}); ok && len(compRows) > 0 {
				if isCompressed, ok := compRows[0]["is_compressed"].(bool); ok {
					ht.CompressionEnabled = isCompressed
				}
			}
		}

		// Check if retention policy is enabled
		retQuery := fmt.Sprintf(
			"SELECT count(*) > 0 as has_retention FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_retention'",
			ht.TableName,
		)
		retResult, err := t.ExecuteSQLWithoutParams(ctx, retQuery)
		if err == nil {
			if retRows, ok := retResult.([]map[string]interface{}); ok && len(retRows) > 0 {
				if hasRetention, ok := retRows[0]["has_retention"].(bool); ok {
					ht.RetentionEnabled = hasRetention
				}
			}
		}

		hypertables = append(hypertables, ht)
	}

	return hypertables, nil
}

// GetHypertable gets information about a specific hypertable
func (t *TimescaleDB) GetHypertable(ctx context.Context, tableName string) (*Hypertable, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf(`
		SELECT h.table_name, h.schema_name, d.column_name as time_column,
			count(d.id) as num_dimensions,
			(
				SELECT column_name FROM _timescaledb_catalog.dimension 
				WHERE hypertable_id = h.id AND column_type != 'TIMESTAMP' 
				AND column_type != 'TIMESTAMPTZ' 
				LIMIT 1
			) as space_column
		FROM _timescaledb_catalog.hypertable h
		JOIN _timescaledb_catalog.dimension d ON h.id = d.hypertable_id
		WHERE h.table_name = '%s'
		GROUP BY h.id, h.table_name, h.schema_name
	`, tableName)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get hypertable information: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", tableName)
	}

	row := rows[0]
	ht := &Hypertable{
		TableName:  fmt.Sprintf("%v", row["table_name"]),
		SchemaName: fmt.Sprintf("%v", row["schema_name"]),
		TimeColumn: fmt.Sprintf("%v", row["time_column"]),
	}

	// Handle nullable columns
	if row["space_column"] != nil {
		ht.SpaceColumn = fmt.Sprintf("%v", row["space_column"])
	}

	// Convert numeric dimensions
	if dimensions, ok := row["num_dimensions"].(int64); ok {
		ht.NumDimensions = int(dimensions)
	} else if dimensions, ok := row["num_dimensions"].(int); ok {
		ht.NumDimensions = dimensions
	}

	// Check if compression is enabled
	compQuery := fmt.Sprintf(
		"SELECT count(*) > 0 as is_compressed FROM timescaledb_information.compression_settings WHERE hypertable_name = '%s'",
		ht.TableName,
	)
	compResult, err := t.ExecuteSQLWithoutParams(ctx, compQuery)
	if err == nil {
		if compRows, ok := compResult.([]map[string]interface{}); ok && len(compRows) > 0 {
			if isCompressed, ok := compRows[0]["is_compressed"].(bool); ok {
				ht.CompressionEnabled = isCompressed
			}
		}
	}

	// Check if retention policy is enabled
	retQuery := fmt.Sprintf(
		"SELECT count(*) > 0 as has_retention FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_retention'",
		ht.TableName,
	)
	retResult, err := t.ExecuteSQLWithoutParams(ctx, retQuery)
	if err == nil {
		if retRows, ok := retResult.([]map[string]interface{}); ok && len(retRows) > 0 {
			if hasRetention, ok := retRows[0]["has_retention"].(bool); ok {
				ht.RetentionEnabled = hasRetention
			}
		}
	}

	return ht, nil
}

// DropHypertable drops a hypertable and all of its chunks
func (t *TimescaleDB) DropHypertable(ctx context.Context, tableName string, cascade bool) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Use DROP TABLE with CASCADE if requested
	query := fmt.Sprintf("DROP TABLE %s", tableName)
	if cascade {
		query += " CASCADE"
	}

	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop hypertable: %w", err)
	}

	return nil
}

// CheckIfHypertable checks if a table is a hypertable
func (t *TimescaleDB) CheckIfHypertable(ctx context.Context, tableName string) (bool, error) {
	if !t.isTimescaleDB {
		return false, fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) > 0 as is_hypertable 
		FROM _timescaledb_catalog.hypertable 
		WHERE table_name = '%s'
	`, tableName)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return false, fmt.Errorf("failed to check if table is a hypertable: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return false, fmt.Errorf("unexpected result from database query")
	}

	isHypertable, ok := rows[0]["is_hypertable"].(bool)
	if !ok {
		return false, fmt.Errorf("unexpected result type from database query")
	}

	return isHypertable, nil
}

// RecentChunks returns information about recent chunks for a hypertable
func (t *TimescaleDB) RecentChunks(ctx context.Context, tableName string, limit int) (interface{}, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Default limit if not provided
	if limit <= 0 {
		limit = 10
	}

	query := fmt.Sprintf(`
		SELECT chunk_name, range_start, range_end, is_compressed
		FROM timescaledb_information.chunks
		WHERE hypertable_name = '%s'
		ORDER BY range_end DESC
		LIMIT %d
	`, tableName, limit)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk information: %w", err)
	}

	return result, nil
}
