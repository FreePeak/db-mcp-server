-- Insert sample data for sensor_readings (past 30 days)
INSERT INTO test_data.sensor_readings (time, sensor_id, temperature, humidity, pressure, battery_level, location)
SELECT
    timestamp '2023-01-01 00:00:00' + (i || ' hours')::interval AS time,
    sensor_id,
    20 + 5 * random() AS temperature,      -- 20-25°C
    50 + 30 * random() AS humidity,        -- 50-80%
    1000 + 20 * random() AS pressure,      -- 1000-1020 hPa
    100 - i/100.0 AS battery_level,        -- Decreasing from 100%
    (ARRAY['room', 'kitchen', 'outdoor', 'basement', 'garage'])[1 + floor(random() * 5)::int] AS location
FROM
    generate_series(0, 720) AS i,          -- 30 days hourly data
    generate_series(1, 5) AS sensor_id;    -- 5 sensors

-- Insert sample data for weather_observations (past 90 days)
INSERT INTO test_data.weather_observations (time, station_id, temperature, precipitation, wind_speed, wind_direction, atmospheric_pressure)
SELECT
    timestamp '2023-01-01 00:00:00' + (i || ' hours')::interval AS time,
    station_id,
    15 + 15 * random() AS temperature,     -- 15-30°C
    CASE WHEN random() < 0.3 THEN random() * 5 ELSE 0 END AS precipitation, -- 70% chance of no rain
    random() * 20 AS wind_speed,           -- 0-20 km/h
    random() * 360 AS wind_direction,      -- 0-360 degrees
    1010 + 10 * random() AS atmospheric_pressure -- 1010-1020 hPa
FROM
    generate_series(0, 2160) AS i,         -- 90 days hourly data
    generate_series(1, 3) AS station_id;   -- 3 weather stations

-- Insert sample data for device_metrics (past 14 days at 1 minute intervals)
INSERT INTO test_data.device_metrics (time, device_id, cpu_usage, memory_usage, network_in, network_out, disk_io)
SELECT
    timestamp '2023-01-15 00:00:00' + (i || ' minutes')::interval AS time,
    device_id,
    10 + 70 * random() AS cpu_usage,       -- 10-80%
    20 + 60 * random() AS memory_usage,    -- 20-80%
    random() * 1000 AS network_in,         -- 0-1000 KB/s
    random() * 500 AS network_out,         -- 0-500 KB/s
    random() * 100 AS disk_io              -- 0-100 MB/s
FROM
    generate_series(0, 20160, 60) AS i,    -- 14 days, every 60 minutes (for faster insertion)
    generate_series(1, 10) AS device_id;   -- 10 devices

-- Insert sample data for stock_prices (past 2 years of daily data)
INSERT INTO test_data.stock_prices (time, symbol, open_price, high_price, low_price, close_price, volume)
SELECT
    timestamp '2022-01-01 00:00:00' + (i || ' days')::interval AS time,
    symbol,
    100 + 50 * random() AS open_price,
    100 + 50 * random() + 10 AS high_price,
    100 + 50 * random() - 10 AS low_price,
    100 + 50 * random() AS close_price,
    floor(random() * 10000 + 1000) AS volume
FROM
    generate_series(0, 730) AS i,          -- 2 years of data
    unnest(ARRAY['AAPL', 'MSFT', 'GOOGL', 'AMZN', 'META']) AS symbol;

-- Insert sample data for multi_partition_data
INSERT INTO test_data.multi_partition_data (time, device_id, region, metric_value)
SELECT
    timestamp '2023-01-01 00:00:00' + (i || ' hours')::interval AS time,
    device_id,
    (ARRAY['us-east', 'us-west', 'eu-central', 'ap-south', 'sa-east'])[1 + floor(random() * 5)::int] AS region,
    random() * 100 AS metric_value
FROM
    generate_series(0, 720) AS i,          -- 30 days hourly data
    generate_series(1, 20) AS device_id;   -- 20 devices across regions

-- Insert some regular table data
INSERT INTO test_data.regular_table (name, value, created_at)
SELECT
    'Item ' || i::text AS name,
    random() * 1000 AS value,
    timestamp '2023-01-01 00:00:00' + (i || ' hours')::interval AS created_at
FROM
    generate_series(1, 100) AS i; 