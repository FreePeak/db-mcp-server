package timescale

import (
	"context"
	"errors"
	"testing"
)

func TestCreateHypertable(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test basic hypertable creation
	config := HypertableConfig{
		TableName:         "test_table",
		TimeColumn:        "time",
		ChunkTimeInterval: "1 day",
		CreateIfNotExists: true,
	}

	err := tsdb.CreateHypertable(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create hypertable: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "create_hypertable")
	AssertQueryContains(t, query, "test_table")
	AssertQueryContains(t, query, "time")
	AssertQueryContains(t, query, "chunk_time_interval")
	AssertQueryContains(t, query, "1 day")

	// Test with partitioning
	config = HypertableConfig{
		TableName:          "test_table",
		TimeColumn:         "time",
		ChunkTimeInterval:  "1 day",
		PartitioningColumn: "device_id",
		SpacePartitions:    4,
	}

	err = tsdb.CreateHypertable(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create hypertable with partitioning: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "create_hypertable")
	AssertQueryContains(t, query, "partition_column")
	AssertQueryContains(t, query, "device_id")
	AssertQueryContains(t, query, "number_partitions")

	// Test with if_not_exists and migrate_data
	config = HypertableConfig{
		TableName:   "test_table",
		TimeColumn:  "time",
		IfNotExists: true,
		MigrateData: true,
	}

	err = tsdb.CreateHypertable(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create hypertable with extra options: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "if_not_exists => TRUE")
	AssertQueryContains(t, query, "migrate_data => TRUE")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.CreateHypertable(ctx, config)
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT create_hypertable(", nil, errors.New("mocked error"))
	err = tsdb.CreateHypertable(ctx, config)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestAddDimension(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test adding a dimension
	err := tsdb.AddDimension(ctx, "test_table", "device_id", 4)
	if err != nil {
		t.Fatalf("Failed to add dimension: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "add_dimension")
	AssertQueryContains(t, query, "test_table")
	AssertQueryContains(t, query, "device_id")
	AssertQueryContains(t, query, "number_partitions => 4")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.AddDimension(ctx, "test_table", "device_id", 4)
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("SELECT add_dimension(", nil, errors.New("mocked error"))
	err = tsdb.AddDimension(ctx, "test_table", "device_id", 4)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestListHypertables(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Prepare mock data
	mockResult := []map[string]interface{}{
		{
			"table_name":     "test_table",
			"schema_name":    "public",
			"time_column":    "time",
			"num_dimensions": 2,
			"space_column":   "device_id",
		},
		{
			"table_name":     "test_table2",
			"schema_name":    "public",
			"time_column":    "timestamp",
			"num_dimensions": 1,
			"space_column":   nil,
		},
	}

	// Register different result patterns for different queries
	mockDB.RegisterQueryResult("FROM _timescaledb_catalog.hypertable h", mockResult, nil)
	mockDB.RegisterQueryResult("FROM timescaledb_information.compression_settings", []map[string]interface{}{
		{"is_compressed": true},
	}, nil)
	mockDB.RegisterQueryResult("FROM timescaledb_information.jobs", []map[string]interface{}{
		{"has_retention": true},
	}, nil)

	// Test listing hypertables
	hypertables, err := tsdb.ListHypertables(ctx)
	if err != nil {
		t.Fatalf("Failed to list hypertables: %v", err)
	}

	// Check the results
	if len(hypertables) != 2 {
		t.Errorf("Expected 2 hypertables, got %d", len(hypertables))
	}

	if hypertables[0].TableName != "test_table" {
		t.Errorf("Expected TableName to be 'test_table', got '%s'", hypertables[0].TableName)
	}

	if hypertables[0].TimeColumn != "time" {
		t.Errorf("Expected TimeColumn to be 'time', got '%s'", hypertables[0].TimeColumn)
	}

	if hypertables[0].SpaceColumn != "device_id" {
		t.Errorf("Expected SpaceColumn to be 'device_id', got '%s'", hypertables[0].SpaceColumn)
	}

	if hypertables[0].NumDimensions != 2 {
		t.Errorf("Expected NumDimensions to be 2, got %d", hypertables[0].NumDimensions)
	}

	if !hypertables[0].CompressionEnabled {
		t.Error("Expected CompressionEnabled to be true, got false")
	}

	if !hypertables[0].RetentionEnabled {
		t.Error("Expected RetentionEnabled to be true, got false")
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.ListHypertables(ctx)
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("FROM _timescaledb_catalog.hypertable h", nil, errors.New("mocked error"))
	_, err = tsdb.ListHypertables(ctx)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestGetHypertable(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Prepare mock data - Set up the correct result by using RegisterQueryResult
	mockResult := []map[string]interface{}{
		{
			"table_name":     "test_table",
			"schema_name":    "public",
			"time_column":    "time",
			"num_dimensions": int64(2),
			"space_column":   "device_id",
		},
	}

	// Register the query result pattern for the main query
	mockDB.RegisterQueryResult("WHERE h.table_name = 'test_table'", mockResult, nil)

	// Register results for the compression check
	mockDB.RegisterQueryResult("FROM timescaledb_information.compression_settings", []map[string]interface{}{
		{"is_compressed": true},
	}, nil)

	// Register results for the retention policy check
	mockDB.RegisterQueryResult("FROM timescaledb_information.jobs", []map[string]interface{}{
		{"has_retention": true},
	}, nil)

	// Test getting a hypertable
	hypertable, err := tsdb.GetHypertable(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to get hypertable: %v", err)
	}

	// Check the results
	if hypertable.TableName != "test_table" {
		t.Errorf("Expected TableName to be 'test_table', got '%s'", hypertable.TableName)
	}

	if hypertable.TimeColumn != "time" {
		t.Errorf("Expected TimeColumn to be 'time', got '%s'", hypertable.TimeColumn)
	}

	if hypertable.SpaceColumn != "device_id" {
		t.Errorf("Expected SpaceColumn to be 'device_id', got '%s'", hypertable.SpaceColumn)
	}

	if hypertable.NumDimensions != 2 {
		t.Errorf("Expected NumDimensions to be 2, got %d", hypertable.NumDimensions)
	}

	if !hypertable.CompressionEnabled {
		t.Error("Expected CompressionEnabled to be true, got false")
	}

	if !hypertable.RetentionEnabled {
		t.Error("Expected RetentionEnabled to be true, got false")
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.GetHypertable(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("WHERE h.table_name = 'test_table'", nil, errors.New("mocked error"))
	_, err = tsdb.GetHypertable(ctx, "test_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}

	// Test table not found - Create a new mock to avoid interference
	newMockDB := NewMockDB()
	newMockDB.SetTimescaleAvailable(true)
	tsdb.Database = newMockDB

	// Register an empty result for the "not_found" table
	newMockDB.RegisterQueryResult("WHERE h.table_name = 'not_found'", []map[string]interface{}{}, nil)
	_, err = tsdb.GetHypertable(ctx, "not_found")
	if err == nil {
		t.Error("Expected error for non-existent table, got nil")
	}
}

func TestDropHypertable(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Test dropping a hypertable
	err := tsdb.DropHypertable(ctx, "test_table", false)
	if err != nil {
		t.Fatalf("Failed to drop hypertable: %v", err)
	}

	// Check that the correct query was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "DROP TABLE test_table")

	// Test dropping with CASCADE
	err = tsdb.DropHypertable(ctx, "test_table", true)
	if err != nil {
		t.Fatalf("Failed to drop hypertable with CASCADE: %v", err)
	}

	// Check that the correct query was executed
	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "DROP TABLE test_table CASCADE")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	err = tsdb.DropHypertable(ctx, "test_table", false)
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("DROP TABLE", nil, errors.New("mocked error"))
	err = tsdb.DropHypertable(ctx, "test_table", false)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}

func TestCheckIfHypertable(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Prepare mock data
	mockResultTrue := []map[string]interface{}{
		{"is_hypertable": true},
	}

	mockResultFalse := []map[string]interface{}{
		{"is_hypertable": false},
	}

	// Test table is a hypertable
	mockDB.RegisterQueryResult("WHERE table_name = 'test_table'", mockResultTrue, nil)
	isHypertable, err := tsdb.CheckIfHypertable(ctx, "test_table")
	if err != nil {
		t.Fatalf("Failed to check if hypertable: %v", err)
	}

	if !isHypertable {
		t.Error("Expected table to be a hypertable, got false")
	}

	// Test table is not a hypertable
	mockDB.RegisterQueryResult("WHERE table_name = 'regular_table'", mockResultFalse, nil)
	isHypertable, err = tsdb.CheckIfHypertable(ctx, "regular_table")
	if err != nil {
		t.Fatalf("Failed to check if hypertable: %v", err)
	}

	if isHypertable {
		t.Error("Expected table not to be a hypertable, got true")
	}

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.CheckIfHypertable(ctx, "test_table")
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("WHERE table_name = 'error_table'", nil, errors.New("mocked error"))
	_, err = tsdb.CheckIfHypertable(ctx, "error_table")
	if err == nil {
		t.Error("Expected query error, got nil")
	}

	// Test unexpected result structure
	mockDB.RegisterQueryResult("WHERE table_name = 'bad_structure'", []map[string]interface{}{}, nil)
	_, err = tsdb.CheckIfHypertable(ctx, "bad_structure")
	if err == nil {
		t.Error("Expected error for bad result structure, got nil")
	}
}

func TestRecentChunks(t *testing.T) {
	mockDB := NewMockDB()
	tsdb := &DB{
		Database:      mockDB,
		isTimescaleDB: true,
	}

	ctx := context.Background()

	// Prepare mock data
	mockResult := []map[string]interface{}{
		{
			"chunk_name":    "_hyper_1_1_chunk",
			"range_start":   "2023-01-01 00:00:00",
			"range_end":     "2023-01-02 00:00:00",
			"is_compressed": false,
		},
		{
			"chunk_name":    "_hyper_1_2_chunk",
			"range_start":   "2023-01-02 00:00:00",
			"range_end":     "2023-01-03 00:00:00",
			"is_compressed": true,
		},
	}

	// Register mock result
	mockDB.RegisterQueryResult("FROM timescaledb_information.chunks", mockResult, nil)

	// Test getting recent chunks
	_, err := tsdb.RecentChunks(ctx, "test_table", 2)
	if err != nil {
		t.Fatalf("Failed to get recent chunks: %v", err)
	}

	// Check that a query with the right table name and limit was executed
	query, _ := mockDB.GetLastQuery()
	AssertQueryContains(t, query, "hypertable_name = 'test_table'")
	AssertQueryContains(t, query, "LIMIT 2")

	// Test with default limit
	_, err = tsdb.RecentChunks(ctx, "test_table", 0)
	if err != nil {
		t.Fatalf("Failed to get recent chunks with default limit: %v", err)
	}

	query, _ = mockDB.GetLastQuery()
	AssertQueryContains(t, query, "LIMIT 10")

	// Test when TimescaleDB is not available
	tsdb.isTimescaleDB = false
	_, err = tsdb.RecentChunks(ctx, "test_table", 2)
	if err == nil {
		t.Error("Expected error when TimescaleDB is not available, got nil")
	}

	// Test execution error
	tsdb.isTimescaleDB = true
	mockDB.RegisterQueryResult("FROM timescaledb_information.chunks", nil, errors.New("mocked error"))
	_, err = tsdb.RecentChunks(ctx, "test_table", 2)
	if err == nil {
		t.Error("Expected query error, got nil")
	}
}
