-- Create continuous aggregate for hourly sensor readings
CREATE MATERIALIZED VIEW test_data.hourly_sensor_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    sensor_id,
    AVG(temperature) AS avg_temp,
    MIN(temperature) AS min_temp,
    MAX(temperature) AS max_temp,
    AVG(humidity) AS avg_humidity,
    AVG(pressure) AS avg_pressure
FROM test_data.sensor_readings
GROUP BY bucket, sensor_id;

-- Create continuous aggregate for daily weather observations
CREATE MATERIALIZED VIEW test_data.daily_weather_stats
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    station_id,
    AVG(temperature) AS avg_temp,
    MIN(temperature) AS min_temp,
    MAX(temperature) AS max_temp,
    SUM(precipitation) AS total_precipitation,
    AVG(wind_speed) AS avg_wind_speed,
    AVG(atmospheric_pressure) AS avg_pressure
FROM test_data.weather_observations
GROUP BY bucket, station_id;

-- Create continuous aggregate for 5-minute device metrics
CREATE MATERIALIZED VIEW test_data.device_metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    device_id,
    AVG(cpu_usage) AS avg_cpu,
    MAX(cpu_usage) AS max_cpu,
    AVG(memory_usage) AS avg_memory,
    MAX(memory_usage) AS max_memory,
    SUM(network_in) AS total_network_in,
    SUM(network_out) AS total_network_out
FROM test_data.device_metrics
GROUP BY bucket, device_id;

-- Create continuous aggregate for monthly stock data
CREATE MATERIALIZED VIEW test_data.monthly_stock_summary
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 month', time) AS bucket,
    symbol,
    FIRST(open_price, time) AS monthly_open,
    MAX(high_price) AS monthly_high,
    MIN(low_price) AS monthly_low,
    LAST(close_price, time) AS monthly_close,
    SUM(volume) AS monthly_volume
FROM test_data.stock_prices
GROUP BY bucket, symbol;

-- Add continuous aggregate policies
SELECT add_continuous_aggregate_policy('test_data.hourly_sensor_stats',
    start_offset => INTERVAL '14 days',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

SELECT add_continuous_aggregate_policy('test_data.daily_weather_stats',
    start_offset => INTERVAL '30 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day');

SELECT add_continuous_aggregate_policy('test_data.device_metrics_5min',
    start_offset => INTERVAL '7 days',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '30 minutes');

SELECT add_continuous_aggregate_policy('test_data.monthly_stock_summary',
    start_offset => INTERVAL '12 months',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day');

-- Enable compression on hypertables
ALTER TABLE test_data.sensor_readings SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'sensor_id,location',
    timescaledb.compress_orderby = 'time DESC'
);

ALTER TABLE test_data.weather_observations SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'station_id',
    timescaledb.compress_orderby = 'time DESC'
);

ALTER TABLE test_data.device_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'device_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Add compression policies
SELECT add_compression_policy('test_data.sensor_readings', INTERVAL '7 days');
SELECT add_compression_policy('test_data.weather_observations', INTERVAL '30 days');
SELECT add_compression_policy('test_data.device_metrics', INTERVAL '3 days');

-- Add retention policies
SELECT add_retention_policy('test_data.sensor_readings', INTERVAL '90 days');
SELECT add_retention_policy('test_data.device_metrics', INTERVAL '30 days'); 