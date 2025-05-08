# TimescaleDB Test Environment

This directory contains initialization scripts for setting up a TimescaleDB test environment with sample data and structures for testing the DB-MCP-Server TimescaleDB integration.

## Overview

The initialization scripts in this directory are executed automatically when the TimescaleDB Docker container starts up. They set up:

1. Required extensions and schemas
2. Sample tables and hypertables for various time-series data types
3. Sample data with realistic patterns
4. Continuous aggregates, compression policies, and retention policies
5. Test users with different permission levels

## Scripts

The scripts are executed in alphabetical order:

- **01-init.sql**: Creates the TimescaleDB extension, test schema, tables, hypertables, and test users
- **02-sample-data.sql**: Populates the tables with sample time-series data
- **03-continuous-aggregates.sql**: Creates continuous aggregates, compression, and retention policies

## Test Data Overview

The test environment includes the following sample datasets:

1. **sensor_readings**: Simulated IoT sensor data with temperature, humidity, pressure readings
2. **weather_observations**: Weather station data with temperature, precipitation, wind readings
3. **device_metrics**: System monitoring data with CPU, memory, network metrics
4. **stock_prices**: Financial time-series data with OHLC price data
5. **multi_partition_data**: Data with both time and space partitioning
6. **regular_table**: Non-hypertable for comparison testing

## Test Users

- **timescale_user**: Main admin user (password: timescale_password)
- **test_readonly**: Read-only access user (password: readonly_password)  
- **test_readwrite**: Read-write access user (password: readwrite_password)

## Usage

This test environment is automatically set up when running:

```
./timescaledb-test.sh start
```

You can access the database directly:

```
psql postgresql://timescale_user:timescale_password@localhost:15435/timescale_test
```

Or through the MCP server:

```
http://localhost:9093
```

## Available Databases in MCP Server

- **timescaledb_test**: Full admin access via timescale_user
- **timescaledb_readonly**: Read-only access via test_readonly user
- **timescaledb_readwrite**: Read-write access via test_readwrite user 