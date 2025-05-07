package timescale

import (
	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// DBConfig extends PostgreSQL configuration with TimescaleDB-specific options
type DBConfig struct {
	// Inherit PostgreSQL config
	PostgresConfig db.Config

	// TimescaleDB-specific settings
	ChunkTimeInterval string             // Default chunk time interval (e.g., "7 days")
	RetentionPolicy   *RetentionPolicy   // Data retention configuration
	CompressionPolicy *CompressionPolicy // Compression configuration
	UseTimescaleDB    bool               // Enable TimescaleDB features (default: true)
}

// RetentionPolicy defines how long to keep data in TimescaleDB
type RetentionPolicy struct {
	Enabled    bool
	Duration   string // e.g., "90 days"
	DropChunks bool   // Whether to physically drop chunks (vs logical deletion)
}

// CompressionPolicy defines how and when to compress data
type CompressionPolicy struct {
	Enabled       bool
	After         string // e.g., "7 days"
	OrderBy       string // Column to order by during compression
	SegmentBy     string // Column to segment by during compression
	CompressChunk bool   // Whether to manually compress chunks
}

// NewDefaultTimescaleDBConfig creates a DBConfig with default values
func NewDefaultTimescaleDBConfig(pgConfig db.Config) DBConfig {
	return DBConfig{
		PostgresConfig:    pgConfig,
		ChunkTimeInterval: "7 days",
		UseTimescaleDB:    true,
		RetentionPolicy: &RetentionPolicy{
			Enabled:    false,
			Duration:   "90 days",
			DropChunks: true,
		},
		CompressionPolicy: &CompressionPolicy{
			Enabled:       false,
			After:         "30 days",
			CompressChunk: true,
		},
	}
}

// IsTimescaleDB returns true if the config is for TimescaleDB
func IsTimescaleDB(config db.Config) bool {
	// TimescaleDB is a PostgreSQL extension, so the driver must be postgres
	if config.Type != "postgres" {
		return false
	}

	// Check if TimescaleDB extension is explicitly enabled in options
	if config.Options != nil {
		if val, ok := config.Options["use_timescaledb"]; ok {
			return val == "true" || val == "1"
		}
	}

	// Default to true for PostgreSQL connections
	return true
}

// FromDBConfig converts a standard db.Config to a DBConfig
func FromDBConfig(config db.Config) DBConfig {
	tsdbConfig := NewDefaultTimescaleDBConfig(config)

	// Override with custom settings from options if present
	if config.Options != nil {
		if val, ok := config.Options["chunk_time_interval"]; ok {
			tsdbConfig.ChunkTimeInterval = val
		}

		if val, ok := config.Options["retention_duration"]; ok {
			tsdbConfig.RetentionPolicy.Duration = val
			tsdbConfig.RetentionPolicy.Enabled = true
		}

		if val, ok := config.Options["compression_after"]; ok {
			tsdbConfig.CompressionPolicy.After = val
			tsdbConfig.CompressionPolicy.Enabled = true
		}

		if val, ok := config.Options["segment_by"]; ok {
			tsdbConfig.CompressionPolicy.SegmentBy = val
		}

		if val, ok := config.Options["order_by"]; ok {
			tsdbConfig.CompressionPolicy.OrderBy = val
		}
	}

	return tsdbConfig
}
