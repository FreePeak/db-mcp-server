-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create sensor data schema
CREATE SCHEMA IF NOT EXISTS test_data;

-- Create sensor_readings table for hypertable tests
CREATE TABLE test_data.sensor_readings (
    time TIMESTAMPTZ NOT NULL,
    sensor_id INTEGER NOT NULL,
    temperature DOUBLE PRECISION,
    humidity DOUBLE PRECISION,
    pressure DOUBLE PRECISION,
    battery_level DOUBLE PRECISION,
    location VARCHAR(50)
);

-- Convert to hypertable
SELECT create_hypertable('test_data.sensor_readings', 'time');

-- Create weather data for continuous aggregate tests
CREATE TABLE test_data.weather_observations (
    time TIMESTAMPTZ NOT NULL,
    station_id INTEGER NOT NULL,
    temperature DOUBLE PRECISION,
    precipitation DOUBLE PRECISION,
    wind_speed DOUBLE PRECISION,
    wind_direction DOUBLE PRECISION,
    atmospheric_pressure DOUBLE PRECISION
);

-- Convert to hypertable
SELECT create_hypertable('test_data.weather_observations', 'time');

-- Create device metrics for compression tests
CREATE TABLE test_data.device_metrics (
    time TIMESTAMPTZ NOT NULL,
    device_id INTEGER NOT NULL,
    cpu_usage DOUBLE PRECISION,
    memory_usage DOUBLE PRECISION,
    network_in DOUBLE PRECISION,
    network_out DOUBLE PRECISION,
    disk_io DOUBLE PRECISION
);

-- Convert to hypertable
SELECT create_hypertable('test_data.device_metrics', 'time');

-- Create stock data for time-series analysis
CREATE TABLE test_data.stock_prices (
    time TIMESTAMPTZ NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    open_price DOUBLE PRECISION,
    high_price DOUBLE PRECISION,
    low_price DOUBLE PRECISION,
    close_price DOUBLE PRECISION,
    volume INTEGER
);

-- Convert to hypertable
SELECT create_hypertable('test_data.stock_prices', 'time');

-- Create a regular table for comparison tests
CREATE TABLE test_data.regular_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    value DOUBLE PRECISION,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create a table for testing space partitioning
CREATE TABLE test_data.multi_partition_data (
    time TIMESTAMPTZ NOT NULL,
    device_id INTEGER NOT NULL,
    region VARCHAR(50),
    metric_value DOUBLE PRECISION
);

-- Convert to hypertable with space partitioning
SELECT create_hypertable(
    'test_data.multi_partition_data', 
    'time', 
    'device_id', 
    number_partitions => 4
);

-- Create test users
CREATE USER test_readonly WITH PASSWORD 'readonly_password';
CREATE USER test_readwrite WITH PASSWORD 'readwrite_password';

-- Grant permissions
GRANT USAGE ON SCHEMA test_data TO test_readonly, test_readwrite;
GRANT SELECT ON ALL TABLES IN SCHEMA test_data TO test_readonly, test_readwrite;
GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA test_data TO test_readwrite;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA test_data TO test_readwrite; 