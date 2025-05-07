package timescale

import (
	"context"
	"fmt"
	"strings"
)

// CompressionSettings represents TimescaleDB compression settings
type CompressionSettings struct {
	HypertableName      string
	SegmentBy           string
	OrderBy             string
	ChunkTimeInterval   string
	CompressionInterval string
	CompressionEnabled  bool
}

// RetentionSettings represents TimescaleDB retention settings
type RetentionSettings struct {
	HypertableName    string
	RetentionInterval string
	RetentionEnabled  bool
}

// EnableCompression enables compression on a hypertable
func (t *TimescaleDB) EnableCompression(ctx context.Context, tableName string, afterInterval string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = true)", tableName)
	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to enable compression: %w", err)
	}

	// Set compression policy if interval is specified
	if afterInterval != "" {
		err = t.AddCompressionPolicy(ctx, tableName, afterInterval, "", "")
		if err != nil {
			return fmt.Errorf("failed to add compression policy: %w", err)
		}
	}

	return nil
}

// DisableCompression disables compression on a hypertable
func (t *TimescaleDB) DisableCompression(ctx context.Context, tableName string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// First, remove any compression policies
	err := t.RemoveCompressionPolicy(ctx, tableName)
	if err != nil {
		return fmt.Errorf("failed to remove compression policy: %w", err)
	}

	// Then disable compression
	query := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = false)", tableName)
	_, err = t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to disable compression: %w", err)
	}

	return nil
}

// AddCompressionPolicy adds a compression policy to a hypertable
func (t *TimescaleDB) AddCompressionPolicy(ctx context.Context, tableName, interval, segmentBy, orderBy string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// First, check if the table has compression enabled
	query := fmt.Sprintf("SELECT compress FROM timescaledb_information.hypertables WHERE hypertable_name = '%s'", tableName)
	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to check compression status: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return fmt.Errorf("table '%s' is not a hypertable", tableName)
	}

	isCompressed := rows[0]["compress"]
	if isCompressed == nil || fmt.Sprintf("%v", isCompressed) == "false" {
		// Enable compression
		enableQuery := fmt.Sprintf("ALTER TABLE %s SET (timescaledb.compress = true)", tableName)
		_, err := t.ExecuteSQLWithoutParams(ctx, enableQuery)
		if err != nil {
			return fmt.Errorf("failed to enable compression: %w", err)
		}
	}

	// Build the compression policy query
	var policyQuery strings.Builder
	policyQuery.WriteString(fmt.Sprintf("SELECT add_compression_policy('%s', INTERVAL '%s'", tableName, interval))

	if segmentBy != "" {
		policyQuery.WriteString(fmt.Sprintf(", segmentby => '%s'", segmentBy))
	}

	if orderBy != "" {
		policyQuery.WriteString(fmt.Sprintf(", orderby => '%s'", orderBy))
	}

	policyQuery.WriteString(")")

	// Add the compression policy
	_, err = t.ExecuteSQLWithoutParams(ctx, policyQuery.String())
	if err != nil {
		return fmt.Errorf("failed to add compression policy: %w", err)
	}

	return nil
}

// RemoveCompressionPolicy removes a compression policy from a hypertable
func (t *TimescaleDB) RemoveCompressionPolicy(ctx context.Context, tableName string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Find the policy ID
	query := fmt.Sprintf(
		"SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_compression'",
		tableName,
	)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to find compression policy: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		// No policy exists, so nothing to remove
		return nil
	}

	// Get the job ID
	jobID := rows[0]["job_id"]
	if jobID == nil {
		return fmt.Errorf("invalid job ID for compression policy")
	}

	// Remove the policy
	removeQuery := fmt.Sprintf("SELECT remove_compression_policy(%v)", jobID)
	_, err = t.ExecuteSQLWithoutParams(ctx, removeQuery)
	if err != nil {
		return fmt.Errorf("failed to remove compression policy: %w", err)
	}

	return nil
}

// GetCompressionSettings gets the compression settings for a hypertable
func (t *TimescaleDB) GetCompressionSettings(ctx context.Context, tableName string) (*CompressionSettings, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	// Check if the table has compression enabled
	query := fmt.Sprintf(
		"SELECT compress FROM timescaledb_information.hypertables WHERE hypertable_name = '%s'",
		tableName,
	)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to check compression status: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("table '%s' is not a hypertable", tableName)
	}

	settings := &CompressionSettings{
		HypertableName: tableName,
	}

	isCompressed := rows[0]["compress"]
	if isCompressed != nil && fmt.Sprintf("%v", isCompressed) == "true" {
		settings.CompressionEnabled = true

		// Get compression-specific settings
		settingsQuery := fmt.Sprintf(
			"SELECT segmentby, orderby FROM timescaledb_information.compression_settings WHERE hypertable_name = '%s'",
			tableName,
		)

		settingsResult, err := t.ExecuteSQLWithoutParams(ctx, settingsQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to get compression settings: %w", err)
		}

		settingsRows, ok := settingsResult.([]map[string]interface{})
		if ok && len(settingsRows) > 0 {
			if segmentBy, ok := settingsRows[0]["segmentby"]; ok && segmentBy != nil {
				settings.SegmentBy = fmt.Sprintf("%v", segmentBy)
			}

			if orderBy, ok := settingsRows[0]["orderby"]; ok && orderBy != nil {
				settings.OrderBy = fmt.Sprintf("%v", orderBy)
			}
		}

		// Check if a compression policy exists
		policyQuery := fmt.Sprintf(
			"SELECT s.schedule_interval, h.chunk_time_interval FROM timescaledb_information.jobs j "+
				"JOIN timescaledb_information.job_stats s ON j.job_id = s.job_id "+
				"JOIN timescaledb_information.hypertables h ON j.hypertable_name = h.hypertable_name "+
				"WHERE j.hypertable_name = '%s' AND j.proc_name = 'policy_compression'",
			tableName,
		)

		policyResult, err := t.ExecuteSQLWithoutParams(ctx, policyQuery)
		if err == nil {
			policyRows, ok := policyResult.([]map[string]interface{})
			if ok && len(policyRows) > 0 {
				if interval, ok := policyRows[0]["schedule_interval"]; ok && interval != nil {
					settings.CompressionInterval = fmt.Sprintf("%v", interval)
				}

				if chunkInterval, ok := policyRows[0]["chunk_time_interval"]; ok && chunkInterval != nil {
					settings.ChunkTimeInterval = fmt.Sprintf("%v", chunkInterval)
				}
			}
		}
	}

	return settings, nil
}

// AddRetentionPolicy adds a data retention policy to a hypertable
func (t *TimescaleDB) AddRetentionPolicy(ctx context.Context, tableName, interval string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf("SELECT add_retention_policy('%s', INTERVAL '%s')", tableName, interval)
	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to add retention policy: %w", err)
	}

	return nil
}

// RemoveRetentionPolicy removes a data retention policy from a hypertable
func (t *TimescaleDB) RemoveRetentionPolicy(ctx context.Context, tableName string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	// Find the policy ID
	query := fmt.Sprintf(
		"SELECT job_id FROM timescaledb_information.jobs WHERE hypertable_name = '%s' AND proc_name = 'policy_retention'",
		tableName,
	)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to find retention policy: %w", err)
	}

	rows, ok := result.([]map[string]interface{})
	if !ok || len(rows) == 0 {
		// No policy exists, so nothing to remove
		return nil
	}

	// Get the job ID
	jobID := rows[0]["job_id"]
	if jobID == nil {
		return fmt.Errorf("invalid job ID for retention policy")
	}

	// Remove the policy
	removeQuery := fmt.Sprintf("SELECT remove_retention_policy(%v)", jobID)
	_, err = t.ExecuteSQLWithoutParams(ctx, removeQuery)
	if err != nil {
		return fmt.Errorf("failed to remove retention policy: %w", err)
	}

	return nil
}

// GetRetentionSettings gets the retention settings for a hypertable
func (t *TimescaleDB) GetRetentionSettings(ctx context.Context, tableName string) (*RetentionSettings, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	settings := &RetentionSettings{
		HypertableName: tableName,
	}

	// Check if a retention policy exists
	query := fmt.Sprintf(
		"SELECT s.schedule_interval FROM timescaledb_information.jobs j "+
			"JOIN timescaledb_information.job_stats s ON j.job_id = s.job_id "+
			"WHERE j.hypertable_name = '%s' AND j.proc_name = 'policy_retention'",
		tableName,
	)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return settings, nil // Return empty settings with no error
	}

	rows, ok := result.([]map[string]interface{})
	if ok && len(rows) > 0 {
		settings.RetentionEnabled = true
		if interval, ok := rows[0]["schedule_interval"]; ok && interval != nil {
			settings.RetentionInterval = fmt.Sprintf("%v", interval)
		}
	}

	return settings, nil
}

// CompressChunks compresses chunks for a hypertable
func (t *TimescaleDB) CompressChunks(ctx context.Context, tableName, olderThan string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	var query string
	if olderThan == "" {
		// Compress all chunks
		query = fmt.Sprintf("SELECT compress_chunks(hypertable => '%s')", tableName)
	} else {
		// Compress chunks older than the specified interval
		query = fmt.Sprintf("SELECT compress_chunks(hypertable => '%s', older_than => INTERVAL '%s')",
			tableName, olderThan)
	}

	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to compress chunks: %w", err)
	}

	return nil
}

// DecompressChunks decompresses chunks for a hypertable
func (t *TimescaleDB) DecompressChunks(ctx context.Context, tableName, newerThan string) error {
	if !t.isTimescaleDB {
		return fmt.Errorf("TimescaleDB extension not available")
	}

	var query string
	if newerThan == "" {
		// Decompress all chunks
		query = fmt.Sprintf("SELECT decompress_chunks(hypertable => '%s')", tableName)
	} else {
		// Decompress chunks newer than the specified interval
		query = fmt.Sprintf("SELECT decompress_chunks(hypertable => '%s', newer_than => INTERVAL '%s')",
			tableName, newerThan)
	}

	_, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to decompress chunks: %w", err)
	}

	return nil
}

// GetChunkCompressionStats gets compression statistics for a hypertable
func (t *TimescaleDB) GetChunkCompressionStats(ctx context.Context, tableName string) (interface{}, error) {
	if !t.isTimescaleDB {
		return nil, fmt.Errorf("TimescaleDB extension not available")
	}

	query := fmt.Sprintf(`
		SELECT
			chunk_name,
			range_start,
			range_end,
			is_compressed,
			before_compression_total_bytes,
			after_compression_total_bytes,
			CASE
				WHEN before_compression_total_bytes = 0 THEN 0
				ELSE (1 - (after_compression_total_bytes::float / before_compression_total_bytes::float)) * 100
			END AS compression_ratio
		FROM timescaledb_information.chunks
		WHERE hypertable_name = '%s'
		ORDER BY range_end DESC
	`, tableName)

	result, err := t.ExecuteSQLWithoutParams(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk compression statistics: %w", err)
	}

	return result, nil
}
