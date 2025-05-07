package timescale

import (
	"testing"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func TestNewDefaultTimescaleDBConfig(t *testing.T) {
	// Create a PostgreSQL config
	pgConfig := db.Config{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
	}

	// Get default TimescaleDB config
	tsdbConfig := NewDefaultTimescaleDBConfig(pgConfig)

	// Check that the PostgreSQL config was correctly embedded
	if tsdbConfig.PostgresConfig.Type != pgConfig.Type {
		t.Errorf("Expected PostgresConfig.Type to be %s, got %s", pgConfig.Type, tsdbConfig.PostgresConfig.Type)
	}
	if tsdbConfig.PostgresConfig.Host != pgConfig.Host {
		t.Errorf("Expected PostgresConfig.Host to be %s, got %s", pgConfig.Host, tsdbConfig.PostgresConfig.Host)
	}
	if tsdbConfig.PostgresConfig.Port != pgConfig.Port {
		t.Errorf("Expected PostgresConfig.Port to be %d, got %d", pgConfig.Port, tsdbConfig.PostgresConfig.Port)
	}
	if tsdbConfig.PostgresConfig.User != pgConfig.User {
		t.Errorf("Expected PostgresConfig.User to be %s, got %s", pgConfig.User, tsdbConfig.PostgresConfig.User)
	}
	if tsdbConfig.PostgresConfig.Password != pgConfig.Password {
		t.Errorf("Expected PostgresConfig.Password to be %s, got %s", pgConfig.Password, tsdbConfig.PostgresConfig.Password)
	}
	if tsdbConfig.PostgresConfig.Name != pgConfig.Name {
		t.Errorf("Expected PostgresConfig.Name to be %s, got %s", pgConfig.Name, tsdbConfig.PostgresConfig.Name)
	}

	// Check default values
	if !tsdbConfig.UseTimescaleDB {
		t.Error("Expected UseTimescaleDB to be true, got false")
	}
	if tsdbConfig.ChunkTimeInterval != "7 days" {
		t.Errorf("Expected ChunkTimeInterval to be '7 days', got '%s'", tsdbConfig.ChunkTimeInterval)
	}
	if tsdbConfig.RetentionPolicy == nil {
		t.Fatal("Expected RetentionPolicy to be non-nil")
	}
	if tsdbConfig.RetentionPolicy.Enabled {
		t.Error("Expected RetentionPolicy.Enabled to be false, got true")
	}
	if tsdbConfig.RetentionPolicy.Duration != "90 days" {
		t.Errorf("Expected RetentionPolicy.Duration to be '90 days', got '%s'", tsdbConfig.RetentionPolicy.Duration)
	}
	if !tsdbConfig.RetentionPolicy.DropChunks {
		t.Error("Expected RetentionPolicy.DropChunks to be true, got false")
	}
	if tsdbConfig.CompressionPolicy == nil {
		t.Fatal("Expected CompressionPolicy to be non-nil")
	}
	if tsdbConfig.CompressionPolicy.Enabled {
		t.Error("Expected CompressionPolicy.Enabled to be false, got true")
	}
	if tsdbConfig.CompressionPolicy.After != "30 days" {
		t.Errorf("Expected CompressionPolicy.After to be '30 days', got '%s'", tsdbConfig.CompressionPolicy.After)
	}
	if !tsdbConfig.CompressionPolicy.CompressChunk {
		t.Error("Expected CompressionPolicy.CompressChunk to be true, got false")
	}
}

func TestIsTimescaleDB(t *testing.T) {
	testCases := []struct {
		name     string
		config   db.Config
		expected bool
	}{
		{
			name: "PostgresWithExplicitTimescaleTrue",
			config: db.Config{
				Type: "postgres",
				Options: map[string]string{
					"use_timescaledb": "true",
				},
			},
			expected: true,
		},
		{
			name: "PostgresWithExplicitTimescaleOne",
			config: db.Config{
				Type: "postgres",
				Options: map[string]string{
					"use_timescaledb": "1",
				},
			},
			expected: true,
		},
		{
			name: "PostgresWithExplicitTimescaleFalse",
			config: db.Config{
				Type: "postgres",
				Options: map[string]string{
					"use_timescaledb": "false",
				},
			},
			expected: false,
		},
		{
			name: "PostgresWithoutExplicitTimescale",
			config: db.Config{
				Type: "postgres",
			},
			expected: true,
		},
		{
			name: "NotPostgres",
			config: db.Config{
				Type: "mysql",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsTimescaleDB(tc.config)
			if result != tc.expected {
				t.Errorf("IsTimescaleDB(%v) = %v, expected %v", tc.config, result, tc.expected)
			}
		})
	}
}

func TestFromDBConfig(t *testing.T) {
	// Create a PostgreSQL config with TimescaleDB options
	pgConfig := db.Config{
		Type:     "postgres",
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		Options: map[string]string{
			"chunk_time_interval": "1 day",
			"retention_duration":  "60 days",
			"compression_after":   "14 days",
			"segment_by":          "device_id",
			"order_by":            "time DESC",
		},
	}

	// Convert to TimescaleDB config
	tsdbConfig := FromDBConfig(pgConfig)

	// Check that options were applied correctly
	if tsdbConfig.ChunkTimeInterval != "1 day" {
		t.Errorf("Expected ChunkTimeInterval to be '1 day', got '%s'", tsdbConfig.ChunkTimeInterval)
	}
	if !tsdbConfig.RetentionPolicy.Enabled {
		t.Error("Expected RetentionPolicy.Enabled to be true, got false")
	}
	if tsdbConfig.RetentionPolicy.Duration != "60 days" {
		t.Errorf("Expected RetentionPolicy.Duration to be '60 days', got '%s'", tsdbConfig.RetentionPolicy.Duration)
	}
	if !tsdbConfig.CompressionPolicy.Enabled {
		t.Error("Expected CompressionPolicy.Enabled to be true, got false")
	}
	if tsdbConfig.CompressionPolicy.After != "14 days" {
		t.Errorf("Expected CompressionPolicy.After to be '14 days', got '%s'", tsdbConfig.CompressionPolicy.After)
	}
	if tsdbConfig.CompressionPolicy.SegmentBy != "device_id" {
		t.Errorf("Expected CompressionPolicy.SegmentBy to be 'device_id', got '%s'", tsdbConfig.CompressionPolicy.SegmentBy)
	}
	if tsdbConfig.CompressionPolicy.OrderBy != "time DESC" {
		t.Errorf("Expected CompressionPolicy.OrderBy to be 'time DESC', got '%s'", tsdbConfig.CompressionPolicy.OrderBy)
	}
}
