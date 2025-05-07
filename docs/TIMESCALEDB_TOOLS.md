# TimescaleDB Tools: Time-Series and Continuous Aggregates

This document provides information about the time-series query tools and continuous aggregate functionality for TimescaleDB in the DB-MCP-Server.

## Time-Series Query Tools

TimescaleDB extends PostgreSQL with specialized time-series capabilities. The DB-MCP-Server includes tools for efficiently working with time-series data.

### Available Tools

| Tool | Description |
|------|-------------|
| `time_series_query` | Execute time-series queries with optimized bucketing |
| `analyze_time_series` | Analyze time-series data patterns and characteristics |

### Time-Series Query Options

The `time_series_query` tool supports the following parameters:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `target_table` | Yes | Table containing time-series data |
| `time_column` | Yes | Column containing timestamp data |
| `bucket_interval` | Yes | Time bucket interval (e.g., '1 hour', '1 day') |
| `start_time` | No | Start of time range (e.g., '2023-01-01') |
| `end_time` | No | End of time range (e.g., '2023-01-31') |
| `aggregations` | No | Comma-separated list of aggregations (e.g., 'AVG(temp),MAX(temp),COUNT(*)') |
| `where_condition` | No | Additional WHERE conditions |
| `group_by` | No | Additional GROUP BY columns (comma-separated) |
| `limit` | No | Maximum number of rows to return |

### Examples

#### Basic Time-Series Query

```json
{
  "operation": "time_series_query",
  "target_table": "sensor_data",
  "time_column": "timestamp",
  "bucket_interval": "1 hour",
  "start_time": "2023-01-01",
  "end_time": "2023-01-02",
  "aggregations": "AVG(temperature) as avg_temp, MAX(temperature) as max_temp"
}
```

#### Query with Additional Filtering and Grouping

```json
{
  "operation": "time_series_query",
  "target_table": "sensor_data",
  "time_column": "timestamp",
  "bucket_interval": "1 day",
  "where_condition": "sensor_id IN (1, 2, 3)",
  "group_by": "sensor_id",
  "limit": 100
}
```

#### Analyzing Time-Series Data Patterns

```json
{
  "operation": "analyze_time_series",
  "target_table": "sensor_data",
  "time_column": "timestamp",
  "start_time": "2023-01-01",
  "end_time": "2023-12-31"
}
```

## Continuous Aggregate Tools

Continuous aggregates are one of TimescaleDB's most powerful features, providing materialized views that automatically refresh as new data is added.

### Available Tools

| Tool | Description |
|------|-------------|
| `create_continuous_aggregate` | Create a new continuous aggregate view |
| `refresh_continuous_aggregate` | Manually refresh a continuous aggregate |

### Continuous Aggregate Options

The `create_continuous_aggregate` tool supports the following parameters:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `view_name` | Yes | Name for the continuous aggregate view |
| `source_table` | Yes | Source table with raw data |
| `time_column` | Yes | Time column to bucket |
| `bucket_interval` | Yes | Time bucket interval (e.g., '1 hour', '1 day') |
| `aggregations` | No | Comma-separated list of aggregations |
| `where_condition` | No | WHERE condition to filter source data |
| `with_data` | No | Whether to materialize data immediately (default: false) |
| `refresh_policy` | No | Whether to add a refresh policy (default: false) |
| `refresh_interval` | No | Refresh interval (e.g., '1 day') |

The `refresh_continuous_aggregate` tool supports:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `view_name` | Yes | Name of the continuous aggregate view |
| `start_time` | No | Start of time range to refresh |
| `end_time` | No | End of time range to refresh |

### Examples

#### Creating a Daily Temperature Aggregate

```json
{
  "operation": "create_continuous_aggregate",
  "view_name": "daily_temperatures",
  "source_table": "sensor_data",
  "time_column": "timestamp",
  "bucket_interval": "1 day",
  "aggregations": "AVG(temperature) as avg_temp, MIN(temperature) as min_temp, MAX(temperature) as max_temp, COUNT(*) as reading_count",
  "with_data": true,
  "refresh_policy": true,
  "refresh_interval": "1 hour"
}
```

#### Refreshing a Continuous Aggregate for a Specific Period

```json
{
  "operation": "refresh_continuous_aggregate",
  "view_name": "daily_temperatures",
  "start_time": "2023-01-01",
  "end_time": "2023-01-31"
}
```

## Common Time Bucket Intervals

TimescaleDB supports various time bucket intervals for grouping time-series data:

- `1 minute`, `5 minutes`, `10 minutes`, `15 minutes`, `30 minutes`
- `1 hour`, `2 hours`, `3 hours`, `6 hours`, `12 hours`
- `1 day`, `1 week`
- `1 month`, `3 months`, `6 months`, `1 year`

## Best Practices

1. **Choose the right bucket interval**: Select a time bucket interval appropriate for your data density and query patterns. Smaller intervals provide more granularity but create more records.

2. **Use continuous aggregates for frequently queried time ranges**: If you often query for daily or monthly aggregates, create continuous aggregates at those intervals.

3. **Add appropriate indexes**: For optimal query performance, ensure your time column is properly indexed, especially on the raw data table.

4. **Consider retention policies**: Use TimescaleDB's retention policies to automatically drop old data from raw tables while keeping aggregated views.

5. **Refresh policies**: Set refresh policies based on how frequently your data is updated and how current your aggregate views need to be. 