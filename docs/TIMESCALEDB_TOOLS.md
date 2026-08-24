# TimescaleDB Tools: Time-Series and Continuous Aggregates

This document provides information about the time-series query tools and continuous aggregate functionality for TimescaleDB in the DB-MCP-Server.

## Time-Series Query Tools

TimescaleDB extends PostgreSQL with specialized time-series capabilities. The DB-MCP-Server includes tools for efficiently working with time-series data.

### Available Tools

These tools are registered automatically for PostgreSQL databases whose config
type is `postgres` (registration is config-driven, so it also works under
`--lazy-loading`; each handler verifies the extension at call time and returns
an actionable error when it is absent). Per-database mode names them
`timescaledb_<tool>_<db_id>`; unified mode drops both affixes in favor of a
required `database` parameter.

| Tool | Description |
|------|-------------|
| `timescaledb_timeseries_query` | Execute time-series queries with optimized bucketing (`time_bucket`) |
| `timescaledb_analyze_timeseries` | Analyze time-series data patterns and characteristics |
| `timescaledb_list_hypertables` | List hypertables with their time column and dimension count (read-only) |
| `timescaledb_compression_settings` | Show compression configuration for one hypertable (read-only; takes `target_table`) |
| `timescaledb_retention_policy` | Show configured retention policy for one hypertable (read-only; takes `target_table`) |
| `timescaledb_list_continuous_aggregates` | List continuous aggregates with bucket interval and refresh policy (read-only) |
| `timescaledb_continuous_aggregate_info` | Inspect one continuous aggregate in detail (read-only; takes `view_name`) |

All read-only discovery above runs through the query pipeline, so it stays
usable on `read_only` databases.

### Time-Series Query Options

The `timescaledb_timeseries_query` tool supports the following parameters:

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

## Continuous Aggregate Discovery Tools

Continuous aggregates are one of TimescaleDB's most powerful features, providing materialized views that automatically refresh as new data is added.

### Available Tools (read-only)

| Tool | Description |
|------|-------------|
| `timescaledb_list_continuous_aggregates_<db_id>` | List continuous aggregates with bucket interval and refresh policy (no parameters) |
| `timescaledb_continuous_aggregate_info_<db_id>` | Inspect one continuous aggregate in detail; requires `view_name` |

> **Scope note**: write-policy operations (`create_hypertable`,
> `enable_compression` / `disable_compression`, `add_compression_policy` /
> `remove_compression_policy`, `add_retention_policy` /
> `remove_retention_policy`, `create_continuous_aggregate`,
> `refresh_continuous_aggregate`, `drop_continuous_aggregate`,
> `add_continuous_aggregate_policy` /
> `remove_continuous_aggregate_policy`) are implemented in the codebase but
> not exposed as MCP tools — use plain SQL through the query/execute tools
> in the meantime. The read-only discovery tools above go through the query
> pipeline and therefore remain usable on `read_only` databases.

### Continuous Aggregate Options

The `timescaledb_list_continuous_aggregates_<db_id>` tool takes no parameters
and returns every continuous aggregate with its bucket interval and refresh
policy.

The `timescaledb_continuous_aggregate_info_<db_id>` tool supports:

| Parameter | Required | Description |
|-----------|----------|-------------|
| `view_name` | Yes | Name of the continuous aggregate view |

### Examples

#### Listing Continuous Aggregates

```json
{
  "operation": "list_continuous_aggregates"
}
```

#### Inspecting One Continuous Aggregate

```json
{
  "operation": "get_continuous_aggregate_info",
  "view_name": "daily_temperatures"
}
```

#### Creating and Refreshing an Aggregate via SQL

Since create/refresh operations are not exposed as tools yet, use the query
and execute tools directly:

```sql
CREATE MATERIALIZED VIEW daily_temperatures
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 day', ts) AS day,
       AVG(temperature) AS avg_temp,
       MAX(temperature) AS max_temp,
       COUNT(*) AS reading_count
FROM sensor_data GROUP BY day;
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