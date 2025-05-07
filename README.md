<div align="center">

<img src="assets/logo.svg" alt="DB MCP Server Logo" width="300" />

# Multi Database MCP Server

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/FreePeak/db-mcp-server)](https://goreportcard.com/report/github.com/FreePeak/db-mcp-server)
[![Go Reference](https://pkg.go.dev/badge/github.com/FreePeak/db-mcp-server.svg)](https://pkg.go.dev/github.com/FreePeak/db-mcp-server)
[![Contributors](https://img.shields.io/github/contributors/FreePeak/db-mcp-server)](https://github.com/FreePeak/db-mcp-server/graphs/contributors)

<h3>A powerful multi-database server implementing the Model Context Protocol (MCP) to provide AI assistants with structured access to databases.</h3>

<div class="toc">
  <a href="#what-is-db-mcp-server">Overview</a> •
  <a href="#core-concepts">Core Concepts</a> •
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#running-the-server">Running</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#available-tools">Tools</a> •
  <a href="#examples">Examples</a> •
  <a href="#troubleshooting">Troubleshooting</a> •
  <a href="#contributing">Contributing</a>
</div>

</div>

## What is DB MCP Server?

The DB MCP Server provides a standardized way for AI models to interact with multiple databases simultaneously. Built on the [FreePeak/cortex](https://github.com/FreePeak/cortex) framework, it enables AI assistants to execute SQL queries, manage transactions, explore schemas, and analyze performance across different database systems through a unified interface.

## Core Concepts

### Multi-Database Support

Unlike traditional database connectors, DB MCP Server can connect to and interact with multiple databases concurrently:

```json
{
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "localhost",
      "port": 3306,
      "name": "db1",
      "user": "user1",
      "password": "password1"
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "localhost",
      "port": 5432,
      "name": "db2",
      "user": "user2",
      "password": "password2"
    }
  ]
}
```

### Dynamic Tool Generation

For each connected database, the server automatically generates specialized tools:

```go
// For a database with ID "mysql1", these tools are generated:
query_mysql1       // Execute SQL queries
execute_mysql1     // Run data modification statements
transaction_mysql1 // Manage transactions
schema_mysql1      // Explore database schema
performance_mysql1 // Analyze query performance
```

### Clean Architecture

The server follows Clean Architecture principles with these layers:

1. **Domain Layer**: Core business entities and interfaces
2. **Repository Layer**: Data access implementations
3. **Use Case Layer**: Application business logic
4. **Delivery Layer**: External interfaces (MCP tools)

## Features

- **Simultaneous Multi-Database Support**: Connect to multiple MySQL and PostgreSQL databases concurrently
- **Database-Specific Tool Generation**: Auto-creates specialized tools for each connected database
- **Clean Architecture**: Modular design with clear separation of concerns
- **OpenAI Agents SDK Compatibility**: Full compatibility for seamless AI assistant integration
- **Dynamic Database Tools**: Execute queries, run statements, manage transactions, explore schemas, analyze performance
- **Unified Interface**: Consistent interaction patterns across different database types
- **Connection Management**: Simple configuration for multiple database connections

## Currently Supported Databases

| Database   | Status                    | Features                                                     |
| ---------- | ------------------------- | ------------------------------------------------------------ |
| MySQL      | ✅ Full Support           | Queries, Transactions, Schema Analysis, Performance Insights |
| PostgreSQL | ✅ Full Support (v9.6-17) | Queries, Transactions, Schema Analysis, Performance Insights |
| TimescaleDB| ✅ Full Support           | Hypertables, Time-Series Queries, Continuous Aggregates, Compression, Retention Policies |

## Quick Start

### Using Docker

```bash
# Pull the latest image
docker pull freepeak/db-mcp-server:latest

# Run with environment variables
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  -e TRANSPORT_MODE=sse \
  -e CONFIG_PATH=/app/my-config.json \
  freepeak/db-mcp-server
```

> **Note**: We mount to `/app/my-config.json` because the container already has a file at `/app/config.json`.

### From Source

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Build the server
make build

# Run the server in SSE mode
./bin/server -t sse -c config.json
```

## Running the Server

The server supports multiple transport modes:

### STDIO Mode (for IDE integration)

```bash
# Run the server in STDIO mode
./server -t stdio -c config.json
```

For Cursor integration, add this to your `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "stdio-db-mcp-server": {
      "command": "/path/to/db-mcp-server/server",
      "args": ["-t", "stdio", "-c", "/path/to/config.json"]
    }
  }
}
```

### SSE Mode (Server-Sent Events)

```bash
# Run with default host (localhost) and port (9092)
./server -t sse -c config.json

# Specify a custom host and port
./server -t sse -host 0.0.0.0 -port 8080 -c config.json
```

Connect your client to `http://localhost:9092/sse` for the event stream.

## Configuration

### Database Configuration

Create a `config.json` file with your database connections:

```json
{
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "mysql1",
      "port": 3306,
      "name": "db1",
      "user": "user1",
      "password": "password1",
      "query_timeout": 60,
      "max_open_conns": 20,
      "max_idle_conns": 5,
      "conn_max_lifetime_seconds": 300,
      "conn_max_idle_time_seconds": 60
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "postgres1",
      "port": 5432,
      "name": "db1",
      "user": "user1",
      "password": "password1"
    }
  ]
}
```

### Command-Line Options

```bash
make build
# Basic options
./bin/server -t <transport> -c <config-file>

# For SSE transport, additional options:
./bin/server -t sse -host <hostname> -port <port> -c <config-file>

# Direct database configuration:
./bin/server -t stdio -db-config '{"connections":[...]}'

# Environment variable configuration:
export DB_CONFIG='{"connections":[...]}'
./server -t stdio
```

## Available Tools

For each connected database, DB MCP Server automatically generates these specialized tools:

### Query Tools

| Tool Name | Description |
|-----------|-------------|
| `query_<db_id>` | Execute SELECT queries and get results as a tabular dataset |
| `execute_<db_id>` | Run data manipulation statements (INSERT, UPDATE, DELETE) |
| `transaction_<db_id>` | Begin, commit, and rollback transactions |

### Schema Tools

| Tool Name | Description |
|-----------|-------------|
| `schema_<db_id>` | Get information about tables, columns, indexes, and foreign keys |
| `generate_schema_<db_id>` | Generate SQL or code from database schema |

### Performance Tools

| Tool Name | Description |
|-----------|-------------|
| `performance_<db_id>` | Analyze query performance and get optimization suggestions |

### TimescaleDB Tools

For PostgreSQL databases with TimescaleDB extension, these additional specialized tools are available:

| Tool Name | Description |
|-----------|-------------|
| `timescaledb_<db_id>` | Perform general TimescaleDB operations |
| `create_hypertable_<db_id>` | Convert a standard table to a TimescaleDB hypertable |
| `list_hypertables_<db_id>` | List all hypertables in the database |
| `time_series_query_<db_id>` | Execute optimized time-series queries with bucketing |
| `time_series_analyze_<db_id>` | Analyze time-series data patterns |
| `continuous_aggregate_<db_id>` | Create materialized views that automatically update |
| `refresh_continuous_aggregate_<db_id>` | Manually refresh continuous aggregates |

For detailed documentation on TimescaleDB tools, see [TIMESCALEDB_TOOLS.md](docs/TIMESCALEDB_TOOLS.md).

## Examples

### Querying Multiple Databases

```