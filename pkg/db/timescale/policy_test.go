package timescale

import (
	"context"
	"errors"
	"testing"
)

func TestEnableCompression(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Register mock responses for checking if the table is a hypertable
	mockDB.RegisterQueryResult("WHERE table_name = 'test_table'", []map[string]interface{}{
		{"is_hypertable": true},
	}, nil)

	// Register mock response for the compression check in timescaledb_information.hypertables
	mockDB.RegisterQueryResult("FROM timescaledb_information.hypertables WHERE hypertable_name", []map[string]interface{}{
		{"compress": true},
	}, nil)

	// Test enabling compression without interval
	err := tsdb.EnableCompression(ctx, "test_table", "")
	if err != nil {
		t.Fatalf("Failed to enable compression: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "ALTER TABLE test_table SET (timescaledb.compress = true)")

	// Test enabling compression with interval
	// Register mock responses for specific queries used in this test
	mockDB.RegisterQueryResult("ALTER TABLE", nil, nil)
	mockDB.RegisterQueryResult("SELECT add_compression_policy", nil, nil)
	mockDB.RegisterQueryResult("timescaledb_information.hypertables WHERE hypertable_name = 'test_table'", []map[string]interface{}{
		{"compress": true},
	}, nil)

	err = tsdb.EnableCompression(ctx, "test_table", "7 days")
	if err != nil {
		t.Fatalf("Failed to enable compression with interval: %v", err)
	}

	// Check that the correct policy query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "add_compression_policy")
	AssertQueryContains(t, query, "test_table")
	AssertQueryContains(t, query, "7 days")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.EnableCompression(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("ALTER TABLE", nil, errors.New("mocked error"))
	err = tsdb.EnableCompression(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestDisableCompression(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock successful policy removal and compression disabling
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{
		{"job_id": 1},
	}, nil)
	mockDB.RegisterQueryResult("SELECT remove_compression_policy", nil, nil)
	mockDB.RegisterQueryResult("ALTER TABLE", nil, nil)

	// Test disabling compression
	err := tsdb.DisableCompression(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to disable compression: %v", err)
	}

	// Check that the correct ALTER TABLE query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "ALTER TABLE test_table SET (timescaledb.compress = false)")

	// Test when no policy exists (should still succeed)
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{}, nil)
	mockDB.RegisterQueryResult("ALTER TABLE", nil, nil)

	err = tsdb.DisableCompression(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to disable compression when no policy exists: %v", err)
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.DisableCompression(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{
		{"job_id": 1},
	}, nil)
	mockDB.RegisterQueryResult("SELECT remove_compression_policy", nil, errors.New("mocked error"))
	err = tsdb.DisableCompression(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestAddCompressionPolicy(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock checking compression status
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", []map[string]interface{}{
		{"compress": true},
	}, nil)

	// Test adding a basic compression policy
	err := tsdb.AddCompressionPolicy(ctx, "test_table", "7 days", "", "")
	if err != nil {
		t.Fatalf("Failed to add compression policy: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT add_compression_policy('test_table', INTERVAL '7 days'")

	// Test adding a policy with segmentby and orderby
	err = tsdb.AddCompressionPolicy(ctx, "test_table", "7 days", "device_id", "time DESC")
	if err != nil {
		t.Fatalf("Failed to add compression policy with additional options: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "segmentby => 'device_id'")
	AssertQueryContains(t, query, "orderby => 'time DESC'")

	// Test enabling compression first if not already enabled
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", []map[string]interface{}{
		{"compress": false},
	}, nil)
	mockDB.RegisterQueryResult("ALTER TABLE", nil, nil)

	err = tsdb.AddCompressionPolicy(ctx, "test_table", "7 days", "", "")
	if err != nil {
		t.Fatalf("Failed to add compression policy with compression enabling: %v", err)
	}

	// Check that the ALTER TABLE query was executed first
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT add_compression_policy")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.AddCompressionPolicy(ctx, "test_table", "7 days", "", "")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error on compression check
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", nil, errors.New("mocked error"))
	err = tsdb.AddCompressionPolicy(ctx, "test_table", "7 days", "", "")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestRemoveCompressionPolicy(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock finding a policy
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{
		{"job_id": 1},
	}, nil)
	mockDB.RegisterQueryResult("SELECT remove_compression_policy", nil, nil)

	// Test removing a compression policy
	err := tsdb.RemoveCompressionPolicy(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to remove compression policy: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT remove_compression_policy")

	// Test when no policy exists (should succeed without error)
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{}, nil)

	err = tsdb.RemoveCompressionPolicy(ctx, "test_table")
	if err != nil {
		t.Errorf("Expected success when no policy exists, got: %v", err)
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.RemoveCompressionPolicy(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", nil, errors.New("mocked error"))
	err = tsdb.RemoveCompressionPolicy(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestGetCompressionSettings(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock compression status check
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", []map[string]interface{}{
		{"compress": true},
	}, nil)

	// Mock compression settings
	mockDB.RegisterQueryResult("SELECT segmentby, orderby FROM timescaledb_information.compression_settings", []map[string]interface{}{
		{"segmentby": "device_id", "orderby": "time DESC"},
	}, nil)

	// Mock policy information
	mockDB.RegisterQueryResult("SELECT s.schedule_interval, h.chunk_time_interval FROM", []map[string]interface{}{
		{"schedule_interval": "7 days", "chunk_time_interval": "1 day"},
	}, nil)

	// Test getting compression settings
	settings, err := tsdb.GetCompressionSettings(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get compression settings: %v", err)
	}

	// Check the returned settings
	if settings.HypertableName != "test_table" {
		t.Errorf("Expected HypertableName to be 'test_table', got '%s'", settings.HypertableName)
	}

	if !settings.CompressionEnabled {
		t.Error("Expected CompressionEnabled to be true, got false")
	}

	if settings.SegmentBy != "device_id" {
		t.Errorf("Expected SegmentBy to be 'device_id', got '%s'", settings.SegmentBy)
	}

	if settings.OrderBy != "time DESC" {
		t.Errorf("Expected OrderBy to be 'time DESC', got '%s'", settings.OrderBy)
	}

	if settings.CompressionInterval != "7 days" {
		t.Errorf("Expected CompressionInterval to be '7 days', got '%s'", settings.CompressionInterval)
	}

	if settings.ChunkTimeInterval != "1 day" {
		t.Errorf("Expected ChunkTimeInterval to be '1 day', got '%s'", settings.ChunkTimeInterval)
	}

	// Test when compression is not enabled
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", []map[string]interface{}{
		{"compress": false},
	}, nil)

	settings, err = tsdb.GetCompressionSettings(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get compression settings when not enabled: %v", err)
	}

	if settings.CompressionEnabled {
		t.Error("Expected CompressionEnabled to be false, got true")
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.GetCompressionSettings(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT compress FROM timescaledb_information.hypertables", nil, errors.New("mocked error"))
	_, err = tsdb.GetCompressionSettings(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestAddRetentionPolicy(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test adding a retention policy
	err := tsdb.AddRetentionPolicy(ctx, "test_table", "30 days")
	if err != nil {
		t.Fatalf("Failed to add retention policy: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT add_retention_policy('test_table', INTERVAL '30 days')")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.AddRetentionPolicy(ctx, "test_table", "30 days")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT add_retention_policy", nil, errors.New("mocked error"))
	err = tsdb.AddRetentionPolicy(ctx, "test_table", "30 days")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestRemoveRetentionPolicy(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock finding a policy
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{
		{"job_id": 1},
	}, nil)
	mockDB.RegisterQueryResult("SELECT remove_retention_policy", nil, nil)

	// Test removing a retention policy
	err := tsdb.RemoveRetentionPolicy(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to remove retention policy: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT remove_retention_policy")

	// Test when no policy exists (should succeed without error)
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", []map[string]interface{}{}, nil)

	err = tsdb.RemoveRetentionPolicy(ctx, "test_table")
	if err != nil {
		t.Errorf("Expected success when no policy exists, got: %v", err)
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.RemoveRetentionPolicy(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT job_id FROM timescaledb_information.jobs", nil, errors.New("mocked error"))
	err = tsdb.RemoveRetentionPolicy(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestGetRetentionSettings(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock policy information
	mockDB.RegisterQueryResult("SELECT s.schedule_interval FROM", []map[string]interface{}{
		{"schedule_interval": "30 days"},
	}, nil)

	// Test getting retention settings
	settings, err := tsdb.GetRetentionSettings(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get retention settings: %v", err)
	}

	// Check the returned settings
	if settings.HypertableName != "test_table" {
		t.Errorf("Expected HypertableName to be 'test_table', got '%s'", settings.HypertableName)
	}

	if !settings.RetentionEnabled {
		t.Error("Expected RetentionEnabled to be true, got false")
	}

	if settings.RetentionInterval != "30 days" {
		t.Errorf("Expected RetentionInterval to be '30 days', got '%s'", settings.RetentionInterval)
	}

	// Test when no policy exists
	mockDB.RegisterQueryResult("SELECT s.schedule_interval FROM", []map[string]interface{}{}, nil)

	settings, err = tsdb.GetRetentionSettings(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get retention settings when no policy exists: %v", err)
	}

	if settings.RetentionEnabled {
		t.Error("Expected RetentionEnabled to be false, got true")
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.GetRetentionSettings(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}
}

func TestCompressChunks(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test compressing all chunks
	err := tsdb.CompressChunks(ctx, "test_table", "")
	if err != nil {
		t.Fatalf("Failed to compress all chunks: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT compress_chunks(hypertable => 'test_table')")

	// Test compressing chunks with older_than specified
	err = tsdb.CompressChunks(ctx, "test_table", "7 days")
	if err != nil {
		t.Fatalf("Failed to compress chunks with older_than: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT compress_chunks(hypertable => 'test_table', older_than => INTERVAL '7 days')")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.CompressChunks(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT compress_chunks", nil, errors.New("mocked error"))
	err = tsdb.CompressChunks(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestDecompressChunks(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test decompressing all chunks
	err := tsdb.DecompressChunks(ctx, "test_table", "")
	if err != nil {
		t.Fatalf("Failed to decompress all chunks: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT decompress_chunks(hypertable => 'test_table')")

	// Test decompressing chunks with newer_than specified
	err = tsdb.DecompressChunks(ctx, "test_table", "7 days")
	if err != nil {
		t.Fatalf("Failed to decompress chunks with newer_than: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "SELECT decompress_chunks(hypertable => 'test_table', newer_than => INTERVAL '7 days')")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.DecompressChunks(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT decompress_chunks", nil, errors.New("mocked error"))
	err = tsdb.DecompressChunks(ctx, "test_table", "")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestGetChunkCompressionStats(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Mock chunk stats
	mockStats := []map[string]interface{}{
		{
			"chunk_name":                     "_hyper_1_1_chunk",
			"range_start":                    "2023-01-01 00:00:00",
			"range_end":                      "2023-01-02 00:00:00",
			"is_compressed":                  true,
			"before_compression_total_bytes": 1000,
			"after_compression_total_bytes":  200,
			"compression_ratio":              80.0,
		},
	}
	mockDB.RegisterQueryResult("FROM timescaledb_information.chunks", mockStats, nil)

	// Test getting chunk compression stats
	_, err := tsdb.GetChunkCompressionStats(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get chunk compression stats: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "FROM timescaledb_information.chunks")
	AssertQueryContains(t, query, "hypertable_name = 'test_table'")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.GetChunkCompressionStats(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("FROM timescaledb_information.chunks", nil, errors.New("mocked error"))
	_, err = tsdb.GetChunkCompressionStats(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}
