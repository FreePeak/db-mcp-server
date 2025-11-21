<div align="center">

<img src="assets/logo.svg" alt="DB MCP Server Logo" width="300" />

# Multi Database MCP Server

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/FreePeak/db-mcp-server)](https://goreportcard.com/report/github.com/FreePeak/db-mcp-server)
[![Go Reference](https://pkg.go.dev/badge/github.com/FreePeak/db-mcp-server.svg)](https://pkg.go.dev/github.com/FreePeak/db-mcp-server)
[![Contributors](https://img.shields.io/github/contributors/FreePeak/db-mcp-server)](https://github.com/FreePeak/db-mcp-server/graphs/contributors)

<h3>A powerful multi-database server implementing the Model Context Protocol (MCP) to provide AI assistants with structured access to databases.</h3>

<div class="toc">
  <a href="#overview">Overview</a> •
  <a href="#core-concepts">Core Concepts</a> •
  <a href="#features">Features</a> •
  <a href="#supported-databases">Supported Databases</a> •
  <a href="#deployment-options">Deployment Options</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#available-tools">Available Tools</a> •
  <a href="#examples">Examples</a> •
  <a href="#troubleshooting">Troubleshooting</a> •
  <a href="#contributing">Contributing</a>
</div>

</div>

## Overview

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

## Supported Databases

| Database   | Status                    | Features                                                     |
| ---------- | ------------------------- | ------------------------------------------------------------ |
| MySQL      | ✅ Full Support           | Queries, Transactions, Schema Analysis, Performance Insights |
| PostgreSQL | ✅ Full Support (v9.6-17) | Queries, Transactions, Schema Analysis, Performance Insights |
| TimescaleDB| ✅ Full Support           | Hypertables, Time-Series Queries, Continuous Aggregates, Compression, Retention Policies |

## Deployment Options

The DB MCP Server can be deployed in multiple ways to suit different environments and integration needs:

### Docker Deployment

```bash
# Pull the latest image
docker pull freepeak/db-mcp-server:latest

# Run with mounted config file
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  -e TRANSPORT_MODE=sse \
  -e CONFIG_PATH=/app/my-config.json \
  freepeak/db-mcp-server
```

> **Note**: Mount to `/app/my-config.json` as the container has a default file at `/app/config.json`.

### STDIO Mode (IDE Integration)

```bash
# Run the server in STDIO mode
./bin/server -t stdio -c config.json
```

For Cursor IDE integration, add to `.cursor/mcp.json`:

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
# Default configuration (localhost:9092)
./bin/server -t sse -c config.json

# Custom host and port
./bin/server -t sse -host 0.0.0.0 -port 8080 -c config.json
```

Client connection endpoint: `http://localhost:9092/sse`

### Source Code Installation

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Build the server
make build

# Run the server
./bin/server -t sse -c config.json
```

## Configuration

### Database Configuration File

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
# Basic syntax
./bin/server -t <transport> -c <config-file>

# SSE transport options
./bin/server -t sse -host <hostname> -port <port> -c <config-file>

# Customize log directory (useful for multi-project setups)
./bin/server -t stdio -c <config-file> -log-dir /tmp/db-mcp-logs

# Inline database configuration
./bin/server -t stdio -db-config '{"connections":[...]}'

# Environment variable configuration
export DB_CONFIG='{"connections":[...]}'
./bin/server -t stdio
```

**Available Flags:**
- `-t, -transport`: Transport mode (`stdio` or `sse`)
- `-c, -config`: Path to database configuration file
- `-p, -port`: Server port for SSE mode (default: 9092)
- `-h, -host`: Server host for SSE mode (default: localhost)
- `-log-level`: Log level (`debug`, `info`, `warn`, `error`)
- `-log-dir`: Directory for log files (default: `./logs` in current directory)
- `-db-config`: Inline JSON database configuration

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

```sql
-- Query the first database
query_mysql1("SELECT * FROM users LIMIT 10")

-- Query the second database in the same context
query_postgres1("SELECT * FROM products WHERE price > 100")
```

### Managing Transactions

```sql
-- Start a transaction
transaction_mysql1("BEGIN")

-- Execute statements within the transaction
execute_mysql1("INSERT INTO orders (customer_id, product_id) VALUES (1, 2)")
execute_mysql1("UPDATE inventory SET stock = stock - 1 WHERE product_id = 2")

-- Commit or rollback
transaction_mysql1("COMMIT")
-- OR
transaction_mysql1("ROLLBACK")
```

### Exploring Database Schema

```sql
-- Get all tables in the database
schema_mysql1("tables")

-- Get columns for a specific table
schema_mysql1("columns", "users")

-- Get constraints
schema_mysql1("constraints", "orders")
```

## Troubleshooting

### Common Issues

- **Connection Failures**: Verify network connectivity and database credentials
- **Permission Errors**: Ensure the database user has appropriate permissions
- **Timeout Issues**: Check the `query_timeout` setting in your configuration

### Logs

Enable verbose logging for troubleshooting:

```bash
./bin/server -t sse -c config.json -v
```

## Contributing

We welcome contributions to the DB MCP Server project! To contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please see our [CONTRIBUTING.md](docs/CONTRIBUTING.md) file for detailed guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.