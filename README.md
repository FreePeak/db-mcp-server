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
- **OpenAI Agents SDK Compatibility**: Full compatibility with the OpenAI Agents SDK for seamless integration with AI assistants
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

# Option 1: Run with environment variables (recommended)
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  -e TRANSPORT_MODE=sse \
  -e CONFIG_PATH=/app/my-config.json \
  freepeak/db-mcp-server

# Option 2: Override the entrypoint
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  --entrypoint /app/server \
  freepeak/db-mcp-server \
  -t sse -c /app/my-config.json

# Option 3: Use shell to execute the command
docker run -p 9092:9092 \
  -v $(pwd)/config.json:/app/my-config.json \
  freepeak/db-mcp-server \
  /bin/sh -c "/app/server -t sse -c /app/my-config.json"
```

> **Note**: We mount to `/app/my-config.json` because the container already has a file at `/app/config.json`.
> If you encounter platform mismatch warnings, you can specify the platform: `--platform linux/amd64` or `--platform linux/arm64`.

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
      - ./config.json:/app/my-config.json
    environment:
      - TRANSPORT_MODE=sse
      - CONFIG_PATH=/app/my-config.json
    # Alternative using entrypoint
    # entrypoint: ["/app/server"]
    # command: ["-t", "sse", "-c", "/app/my-config.json"]
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

### Tool Naming Convention

The server automatically generates tools with names following this format:

```
<tool_type>_<database_id>
```

Where:
- `<tool_type>`: One of: query, execute, transaction, schema, performance
- `<database_id>`: The ID of the database as defined in your configuration

Example tool names for a database with ID "mysql1":
- `query_mysql1`
- `execute_mysql1`
- `transaction_mysql1`
- `schema_mysql1`
- `performance_mysql1`

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

### Global Tools

- `list_databases`: List all configured database connections
  ```json
  {}
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
4. **Docker Volume Mount Errors**: If you see errors like `mountpoint for /app/config.json: not a directory`, it's because the container already has a file at that path. Mount to a different path (e.g., `/app/my-config.json`) and update your configuration accordingly.
5. **Docker Command Errors**: If you encounter command-related errors with Docker, use one of these approaches:
   - Use environment variables: `-e TRANSPORT_MODE=sse -e CONFIG_PATH=/app/my-config.json`
   - Override the entrypoint: `--entrypoint /app/server freepeak/db-mcp-server -t sse -c /app/my-config.json`
   - Use shell execution: `freepeak/db-mcp-server /bin/sh -c "/app/server -t sse -c /app/my-config.json"`

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

## Cursor Integration

### Tool Naming Convention

The MCP server registers tools with names that match the format Cursor expects. The tool names follow this format:

```
mcp_<servername>_<tooltype>_<dbID>
```

For example: `mcp_mysql1_db_mcp_server_stdio_schema_mysql1_db`

The server uses the name `mysql1_db_mcp_server_stdio` by default, which should match your Cursor configuration in the `mcp.json` file.

### Cursor Configuration

In your Cursor configuration (`~/.cursor/mcp.json`), you should have a configuration like:

```json
{
    "mcpServers": {
        "multidb": {
            "command": "/path/to/db-mcp-server/server",
            "args": [
                "-t",
                "stdio",
                "-c",
                "/path/to/database_config.json"
            ]
        }
    }
}
```

The server will automatically register tools with simple names that match the database identifiers in your configuration.

### Using MCP Tools in Cursor

Once your DB MCP Server is running and properly configured in Cursor, you can use the MCP tools in your AI assistant conversations. The tools follow this naming pattern:

```
mcp_<server_name>_<tool_type>_<database_id>
```

Where:
- `<server_name>` is the name defined in your .cursor/mcp.json (e.g., "multidb")
- `<tool_type>` is one of: query, execute, transaction, schema, performance, list_databases
- `<database_id>` is the database ID from your configuration (not needed for list_databases)

#### Examples:

For a server named "multidb" with a database ID "mysql1":

1. **Listing all databases**:
```
mcp_multidb_list_databases
```

2. **Querying the database**:
```
mcp_multidb_query_mysql1
Query: SELECT * FROM users LIMIT 10
```

3. **Viewing database schema**:
```
mcp_multidb_schema_mysql1
```

4. **Executing statements**:
```
mcp_multidb_execute_mysql1
Statement: INSERT INTO users (name, email) VALUES ('John Doe', 'john@example.com')
```

5. **Managing transactions**:
```
mcp_multidb_transaction_mysql1
Action: begin
```

#### Troubleshooting MCP Tools in Cursor

If the AI assistant can't call the MCP tools:

1. Make sure the server is running (check with `ps aux | grep server`)
2. Verify your .cursor/mcp.json configuration is correct
3. Ensure the server_name in .env matches what's in your MCP tool calls
4. Restart Cursor after making configuration changes
5. Check the logs in the logs/ directory for any errors

## OpenAI Agents SDK Integration

The DB MCP Server fully supports OpenAI's Agents SDK, allowing you to create AI agents that can interact with databases directly.

### Prerequisites

- OpenAI account with API access
- OpenAI Agents SDK installed: `pip install openai-agents`
- A running DB MCP Server instance (SSE mode)

### Basic Integration Example

Here's how to integrate the DB MCP Server with an OpenAI Agent:

```python
from openai import OpenAI
from agents.agent import Agent, ModelSettings
from agents.tools.mcp_server import MCPServerSse, MCPServerSseParams

# Connect to the MCP server
db_server = MCPServerSse(
    params=MCPServerSseParams(
        url="http://localhost:9095/sse",  # URL to your running DB MCP server
        schema={
            "params": {
                "type": "array", 
                "items": {
                    "type": "object",
                    "properties": {
                        "name": {"type": "string"},
                        "description": {"type": "string"},
                        "parameters": {"type": "object"}
                    }
                }
            }
        }
    ),
)

# Create the agent with access to database tools
agent = Agent(
    name="Database Agent",
    model="gpt-4o",
    model_settings=ModelSettings(temperature=0.1),
    instructions="""
    You are a database helper agent. You can execute SQL queries,
    manage database transactions, and explore schema information.
    """,
    mcp_servers=[db_server],
)

# Now the agent can be used to interact with your databases through the OpenAI API
```

### Testing Your Integration

The repository includes a test script to verify compatibility with the OpenAI Agents SDK:

```bash
# Run the test script
./test_tools/openai-agent-sdk-test/run_test.sh
```

The script will:
1. Build the server with the latest changes
2. Start the server if it's not already running
3. Test the connection with the OpenAI Agents SDK
4. Report if the integration is working correctly

### Troubleshooting Agents SDK Integration

If you encounter issues:

1. Ensure the server is running in SSE mode on the expected port
2. Check that your OpenAI API key is set as an environment variable
3. Verify that your agent's instructions mention the database tools specifically
4. Inspect the server logs for any error messages