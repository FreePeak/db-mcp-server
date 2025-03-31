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

For each connected database, the server automatically generates a set of specialized tools:

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

- **Simultaneous Multi-Database Support**: Connect to and interact with multiple MySQL and PostgreSQL databases concurrently
- **Database-Specific Tool Generation**: Auto-creates specialized tools for each connected database
- **Clean Architecture**: Modular design with clear separation of concerns
- **Dynamic Database Tools**: 
  - Execute SQL queries with parameters
  - Run data modification statements with proper error handling
  - Manage database transactions across sessions
  - Explore database schemas and relationships
  - Analyze query performance and receive optimization suggestions
- **Unified Interface**: Consistent interaction patterns across different database types
- **Connection Management**: Simple configuration for multiple database connections

## Currently Supported Databases

| Database | Status | Features |
|----------|--------|----------|
| MySQL    | ✅ Full Support | Queries, Transactions, Schema Analysis, Performance Insights |
| PostgreSQL | ✅ Full Support | Queries, Transactions, Schema Analysis, Performance Insights |

## Quick Start

### Using Docker

The quickest way to get started is with Docker:

```bash
# Pull the latest image
docker pull freepeak/db-mcp-server:latest

# Run with your config mounted
docker run -p 9092:9092 -v $(pwd)/config.json:/app/config.json freepeak/db-mcp-server -t sse -c /app/config.json
```

### From Source

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Build the server
make build

# Run the server in SSE mode
./server -t sse -c config.json
```

## Running the Server

The server supports multiple transport modes to fit different use cases:

### STDIO Mode (for IDE integration)

Ideal for integration with AI coding assistants:

```bash
# Run the server in STDIO mode
./server -t stdio -c config.json
```

Output will be sent as JSON-RPC messages to stdout, while logs go to stderr.

For Cursor integration, add this to your `.cursor/mcp.json`:

```json
{
    "mcpServers": {
        "stdio-db-mcp-server": {
            "command": "/path/to/db-mcp-server/server",
            "args": [
                "-t", "stdio",
                "-c", "/path/to/config.json"
            ]
        }
    }
}
```

### SSE Mode (Server-Sent Events)

For web-based applications and services:

```bash
# Run with default host (localhost) and port (9092)
./server -t sse -c config.json

# Specify a custom host and port
./server -t sse -host 0.0.0.0 -port 8080 -c config.json
```

Connect your client to `http://localhost:9092/sse` for the event stream.

### Docker Compose

For development environments with database containers:

```yaml
# docker-compose.yml
version: '3'
services:
  db-mcp-server:
    image: freepeak/db-mcp-server:latest
    ports:
      - "9092:9092"
    volumes:
      - ./config.json:/app/config.json
    command: ["-t", "sse", "-c", "/app/config.json"]
    depends_on:
      - mysql
      - postgres
  
  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: rootpassword
      MYSQL_DATABASE: testdb
      MYSQL_USER: user
      MYSQL_PASSWORD: password
    ports:
      - "3306:3306"
  
  postgres:
    image: postgres:14
    environment:
      POSTGRES_DB: testdb
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
```

## Configuration

### Database Configuration

Create a `config.json` file with your database connections:

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

### Command-Line Options

The server supports various command-line options:

```bash
# Basic options
./server -t <transport> -c <config-file>

# Available transports: stdio, sse
# For SSE transport, additional options:
./server -t sse -host <hostname> -port <port> -c <config-file>

# Direct database configuration:
./server -t stdio -db-config '{"connections":[...]}'

# Environment variable configuration:
export DB_CONFIG='{"connections":[...]}'
./server -t stdio
```

## Available Tools

For each connected database (e.g., "mysql1", "mysql2"), the server creates:

## MCP Tools Naming Convention

The MCP server automatically registers tools with names that are compatible with Cursor's MCP tool naming convention. The naming format is:

```
mcp_<server-name>_<tool-type>_<database-id>
```

For example, if your server name is set to "db" (the default) and you have a database with ID "mysql1", the tools would be:

```
mcp_multidb_query_mysql1
mcp_multidb_execute_mysql1
mcp_multidb_transaction_mysql1
mcp_multidb_performance_mysql1
mcp_multidb_schema_mysql1
```

And the common database tool:

```
mcp_multidb_list_databases
```

### Configuring the Server Name

You can configure the server name (which affects the tool naming prefix) in two ways:

1. Set the `MCP_SERVER_NAME` environment variable:
   ```sh
   export MCP_SERVER_NAME=myserver
   ```

2. Add it to your `.env` file:
   ```
   MCP_SERVER_NAME=myserver
   ```

If not specified, the default server name is "db".

### Matching Cursor Configuration with Server Name

For proper tool naming and functionality, the key in your Cursor's `mcp.json` file should match the `MCP_SERVER_NAME` value in your server configuration:

1. In your `.env` file or environment variables:
```
MCP_SERVER_NAME=multidb  # This defines the prefix for tool names
```

2. In your `.cursor/mcp.json` file, use the same name as the key:
```json
{{
    "mcpServers": {
        "multidb": {
            "command": "path/to/your/server",
            "args": [
                "-t",
                "stdio",
                "-c",
                "path/to/your/database-config.json"
            ],
            "env": {
                "MCP_SERVER_NAME": "multidb"
            }
        }
    }
}
```

With this configuration, tool names will follow the convention `mcp_multidb_<tool-type>_<database-id>`.

For example:
- `mcp_multidb_query_cashflow`
- `mcp_multidb_execute_cashflow`
- `mcp_multidb_transaction_cashflow`
- `mcp_multidb_list_databases`

### Database-Specific Tools

- `query_<dbid>`: Execute SQL queries on the specified database
  ```json
  {
    "query": "SELECT * FROM users WHERE age > ?",
    "params": [30]
  }
  ```

- `execute_<dbid>`: Execute SQL statements (INSERT, UPDATE, DELETE)
  ```json
  {
    "statement": "INSERT INTO users (name, email) VALUES (?, ?)",
    "params": ["John Doe", "john@example.com"]
  }
  ```

- `transaction_<dbid>`: Manage database transactions
  ```json
  // Begin transaction
  {
    "action": "begin",
    "readOnly": false
  }
  
  // Execute within transaction
  {
    "action": "execute",
    "transactionId": "<from begin response>",
    "statement": "UPDATE users SET active = ? WHERE id = ?",
    "params": [true, 42]
  }
  
  // Commit transaction
  {
    "action": "commit",
    "transactionId": "<from begin response>"
  }
  ```

- `schema_<dbid>`: Get database schema information
  ```json
  {
    "random_string": "dummy"
  }
  ```

- `performance_<dbid>`: Analyze query performance
  ```json
  {
    "action": "analyzeQuery",
    "query": "SELECT * FROM users WHERE name LIKE ?"
  }
  ```

## Examples

### Querying Multiple Databases

```json
// Query the first database
{
  "name": "query_mysql1",
  "parameters": {
    "query": "SELECT * FROM users LIMIT 5"
  }
}

// Query the second database
{
  "name": "query_mysql2",
  "parameters": {
    "query": "SELECT * FROM products LIMIT 5"
  }
}
```

### Executing Transactions

```json
// Begin transaction
{
  "name": "transaction_mysql1",
  "parameters": {
    "action": "begin"
  }
}
// Response contains transactionId

// Execute within transaction
{
  "name": "transaction_mysql1",
  "parameters": {
    "action": "execute",
    "transactionId": "tx_12345",
    "statement": "INSERT INTO orders (user_id, product_id) VALUES (?, ?)",
    "params": [1, 2]
  }
}

// Commit transaction
{
  "name": "transaction_mysql1",
  "parameters": {
    "action": "commit",
    "transactionId": "tx_12345"
  }
}
```

## Roadmap

We're committed to expanding DB MCP Server to support a wide range of database systems:

### Q3 2025
- **MongoDB** - Support for document-oriented database operations
- **SQLite** - Lightweight embedded database integration
- **MariaDB** - Complete feature parity with MySQL implementation

### Q4 2025
- **Microsoft SQL Server** - Enterprise database support with T-SQL capabilities
- **Oracle Database** - Enterprise-grade integration
- **Redis** - Key-value store operations

### 2026
- **Cassandra** - Distributed NoSQL database support
- **Elasticsearch** - Specialized search and analytics capabilities
- **CockroachDB** - Distributed SQL database for global-scale applications
- **DynamoDB** - AWS-native NoSQL database integration
- **Neo4j** - Graph database support
- **ClickHouse** - Analytics database support

## Troubleshooting

### Common Issues

1. **Connection Errors**: Verify your database connection settings in `config.json`
2. **Tool Not Found**: Ensure the server is running and check tool name prefixes
3. **Failed Queries**: Check your SQL syntax and database permissions

### Logs

The server writes logs to:
- STDIO mode: stderr
- SSE mode: stdout and `./logs/db-mcp-server.log`

Enable debug logging with the `-debug` flag:

```bash
./server -t sse -debug -c config.json
```

## Contributing

Contributions are welcome! Here's how you can help:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b new-feature`
3. **Commit** your changes: `git commit -am 'Add new feature'` 
4. **Push** to the branch: `git push origin new-feature`
5. **Submit** a pull request

Please ensure your code follows our coding standards and includes appropriate tests.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support & Contact

- For questions or issues, email [mnhatlinh.doan@gmail.com](mailto:mnhatlinh.doan@gmail.com)
- Open an issue directly: [Issue Tracker](https://github.com/FreePeak/db-mcp-server/issues)
- If DB MCP Server helps your work, please consider supporting:

<p align="">
<a href="https://www.buymeacoffee.com/linhdmn">
<img src="https://img.buymeacoffee.com/button-api/?text=Support DB MCP Server&emoji=☕&slug=linhdmn&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" 
alt="Buy Me A Coffee"/>
</a>
</p>