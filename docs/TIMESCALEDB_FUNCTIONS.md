# TimescaleDB Functions Reference

This document provides a comprehensive reference guide for TimescaleDB functions available through DB-MCP-Server. These functions can be used in SQL queries when connected to a TimescaleDB-enabled PostgreSQL database.

## Time Buckets

These functions are used to group time-series data into intervals for aggregation and analysis.

| Function | Description | Parameters | Example |
|----------|-------------|------------|---------|
| `time_bucket(interval, timestamp)` | Groups time into even buckets | `interval`: The bucket size<br>`timestamp`: The timestamp column | `SELECT time_bucket('1 hour', time) AS hour, avg(value) FROM metrics GROUP BY hour` |
| `time_bucket_gapfill(interval, timestamp)` | Creates time buckets with gap filling for missing values | `interval`: The bucket size<br>`timestamp`: The timestamp column | `SELECT time_bucket_gapfill('1 hour', time) AS hour, avg(value) FROM metrics GROUP BY hour` |
| `time_bucket_ng(interval, timestamp, timezone)` | Next-generation time bucketing with timezone support | `interval`: The bucket size<br>`timestamp`: The timestamp column<br>`timezone`: The timezone to use | `SELECT time_bucket_ng('1 day', time, 'UTC') AS day, avg(value) FROM metrics GROUP BY day` |

## Hypertable Management

These functions are used to create and manage hypertables, which are the core partitioned tables in TimescaleDB.

| Function | Description | Parameters | Example |
|----------|-------------|------------|---------|
| `create_hypertable(table_name, time_column)` | Converts a standard PostgreSQL table into a hypertable | `table_name`: The name of the table<br>`time_column`: The name of the time column | `SELECT create_hypertable('metrics', 'time')` |
| `add_dimension(hypertable, column_name)` | Adds another dimension for partitioning | `hypertable`: The hypertable name<br>`column_name`: The column to partition by | `SELECT add_dimension('metrics', 'device_id')` |
| `add_compression_policy(hypertable, older_than)` | Adds an automatic compression policy | `hypertable`: The hypertable name<br>`older_than`: The age threshold for data to be compressed | `SELECT add_compression_policy('metrics', INTERVAL '7 days')` |
| `add_retention_policy(hypertable, drop_after)` | Adds an automatic data retention policy | `hypertable`: The hypertable name<br>`drop_after`: The age threshold for data to be dropped | `SELECT add_retention_policy('metrics', INTERVAL '30 days')` |

## Continuous Aggregates

These functions manage continuous aggregates, which are materialized views that automatically maintain aggregated time-series data.

| Function | Description | Parameters | Example |
|----------|-------------|------------|---------|
| `CREATE MATERIALIZED VIEW ... WITH (timescaledb.continuous)` | Creates a continuous aggregate view | SQL statement defining the view | `CREATE MATERIALIZED VIEW metrics_hourly WITH (timescaledb.continuous) AS SELECT time_bucket('1 hour', time) as hour, avg(value) FROM metrics GROUP BY hour;` |
| `add_continuous_aggregate_policy(view_name, start_offset, end_offset, schedule_interval)` | Adds a refresh policy to a continuous aggregate | `view_name`: The continuous aggregate name<br>`start_offset`: The start of refresh window relative to current time<br>`end_offset`: The end of refresh window relative to current time<br>`schedule_interval`: How often to refresh | `SELECT add_continuous_aggregate_policy('metrics_hourly', INTERVAL '2 days', INTERVAL '1 hour', INTERVAL '1 hour')` |
| `refresh_continuous_aggregate(continuous_aggregate, start_time, end_time)` | Manually refreshes a continuous aggregate | `continuous_aggregate`: The continuous aggregate name<br>`start_time`: Start time to refresh<br>`end_time`: End time to refresh | `SELECT refresh_continuous_aggregate('metrics_hourly', '2023-01-01', '2023-01-02')` |

## Analytics Functions

Special analytics functions provided by TimescaleDB for time-series analysis.

| Function | Description | Parameters | Example |
|----------|-------------|------------|---------|
| `first(value, time)` | Returns the value at the first time | `value`: The value column<br>`time`: The time column | `SELECT first(value, time) FROM metrics GROUP BY device_id` |
| `last(value, time)` | Returns the value at the last time | `value`: The value column<br>`time`: The time column | `SELECT last(value, time) FROM metrics GROUP BY device_id` |
| `time_weight(value, time)` | Calculates time-weighted average | `value`: The value column<br>`time`: The time column | `SELECT time_weight(value, time) FROM metrics GROUP BY device_id` |
| `histogram(value, min, max, num_buckets)` | Creates a histogram of values | `value`: The value column<br>`min`: Minimum bucket value<br>`max`: Maximum bucket value<br>`num_buckets`: Number of buckets | `SELECT histogram(value, 0, 100, 10) FROM metrics` |
| `approx_percentile(value, percentile)` | Calculates approximate percentiles | `value`: The value column<br>`percentile`: The percentile (0.0-1.0) | `SELECT approx_percentile(value, 0.95) FROM metrics` |

## Query Patterns and Best Practices

### Time-Series Aggregation with Buckets

```sql
-- Basic time-series aggregation using time_bucket
SELECT 
  time_bucket('1 hour', time) AS hour,
  avg(temperature) AS avg_temp,
  min(temperature) AS min_temp,
  max(temperature) AS max_temp
FROM sensor_data
WHERE time > now() - INTERVAL '1 day'
GROUP BY hour
ORDER BY hour;

-- Time-series aggregation with gap filling
SELECT 
  time_bucket_gapfill('1 hour', time) AS hour,
  avg(temperature) AS avg_temp,
  min(temperature) AS min_temp,
  max(temperature) AS max_temp
FROM sensor_data
WHERE time > now() - INTERVAL '1 day'
GROUP BY hour
ORDER BY hour;
```

### Working with Continuous Aggregates

```sql
-- Creating a continuous aggregate view
CREATE MATERIALIZED VIEW sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 hour', time) AS hour,
  device_id,
  avg(temperature) AS avg_temp
FROM sensor_data
GROUP BY hour, device_id;

-- Querying a continuous aggregate
SELECT hour, avg_temp 
FROM sensor_data_hourly
WHERE hour > now() - INTERVAL '7 days'
  AND device_id = 'dev001'
ORDER BY hour;
```

### Hypertable Management

```sql
-- Creating a hypertable
CREATE TABLE sensor_data (
  time TIMESTAMPTZ NOT NULL,
  device_id TEXT NOT NULL,
  temperature FLOAT,
  humidity FLOAT
);

SELECT create_hypertable('sensor_data', 'time');

-- Adding a second dimension for partitioning
SELECT add_dimension('sensor_data', 'device_id', number_partitions => 4);

-- Adding compression policy
ALTER TABLE sensor_data SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'device_id'
);

SELECT add_compression_policy('sensor_data', INTERVAL '7 days');

-- Adding retention policy
SELECT add_retention_policy('sensor_data', INTERVAL '90 days');
```

## Performance Optimization Tips

1. **Use appropriate chunk intervals** - For infrequent data, use larger intervals (e.g., 1 day). For high-frequency data, use smaller intervals (e.g., 1 hour).

2. **Leverage SegmentBy in compression** - When compressing data, use the `timescaledb.compress_segmentby` option with columns that are frequently used in WHERE clauses.

3. **Create indexes on commonly queried columns** - In addition to the time index, create indexes on columns used frequently in queries.

4. **Use continuous aggregates for frequently accessed aggregated data** - This pre-computes aggregations and dramatically improves query performance.

5. **Query only the chunks you need** - Always include a time constraint in your queries to limit the data scanned.

## Troubleshooting

Common issues and solutions:

1. **Slow queries** - Check query plans with `EXPLAIN ANALYZE` and ensure you're using appropriate indexes and time constraints.

2. **High disk usage** - Review compression policies and ensure they are running. Check chunk intervals.

3. **Policy jobs not running** - Use `SELECT * FROM timescaledb_information.jobs` to check job status.

4. **Upgrade issues** - Follow TimescaleDB's official documentation for upgrade procedures. 