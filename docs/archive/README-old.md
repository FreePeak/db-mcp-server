<div align="center">

# Multi DB MCP Server

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/FreePeak/db-mcp-server)](https://goreportcard.com/report/github.com/FreePeak/db-mcp-server)
[![Go Reference](https://pkg.go.dev/badge/github.com/FreePeak/db-mcp-server.svg)](https://pkg.go.dev/github.com/FreePeak/db-mcp-server)
[![Contributors](https://img.shields.io/github/contributors/FreePeak/db-mcp-server)](https://github.com/FreePeak/db-mcp-server/graphs/contributors)

<h3>A robust multi-database implementation of the Database Model Context Protocol (DB MCP)</h3>

[Features](#key-features) • [AI Benefits](#ai-integration-benefits) • [Installation](#installation) • [Usage](#usage) • [Documentation](#documentation) • [Contributing](#contributing) • [License](#license)

</div>

---

## 📋 Overview

The DB MCP Server is a high-performance implementation of the Database Model Context Protocol designed to revolutionize how AI agents interact with databases. By creating a standardized communication layer between AI models and database systems, it enables AI agents to discover, understand, and manipulate database structures with unprecedented context awareness. Currently supporting MySQL and PostgreSQL databases, with plans to expand to most widely used databases including NoSQL solutions, DB MCP Server eliminates the knowledge gap between AI agents and your data, enabling more intelligent, context-aware database operations that previously required human expertise.

## ✨ Key Features

- **AI-Optimized Context Protocol**: Provides rich database context to AI agents, enabling them to reason about schema, relationships, and data patterns
- **Semantic Understanding Bridge**: Translates between natural language queries and database operations with full schema awareness
- **Contextual Database Operations**: Allows AI agents to execute database operations with full understanding of schema, constraints, and relationships
- **Multi-Database Support**: Currently supports MySQL and PostgreSQL with plans for expansion
- **Dynamic Tool Registry**: Register, discover, and invoke database tools at runtime via standard protocol AI agents can understand
- **Editor Integration**: First-class support for VS Code and Cursor extensions with AI-aware features
- **Schema-Aware Assistance**: Provides AI models with complete database structure knowledge for better suggestions
- **Performance Insights**: Delivers performance analytics that AI can leverage for optimization recommendations

## 🧠 AI Integration Benefits

The DB MCP Server transforms how AI agents interact with databases in several key ways:

### Enhanced Contextual Understanding
- **Schema Awareness**: AI agents gain complete knowledge of database tables, columns, relationships, and constraints
- **Semantic Relationship Mapping**: Enables AI to understand not just structure but meaning and purpose of data elements
- **Query Context Preservation**: Maintains context between related operations for coherent multi-step reasoning

### Intelligent Database Operations
- **Natural Language to SQL**: Translates user intent into optimized database operations with full schema awareness
- **Context-Aware Query Generation**: Creates queries that respect database structure, types, and relationships
- **Error Prevention**: Understands database constraints before execution, preventing common errors
- **Optimization Suggestions**: Provides AI with execution metrics for intelligent query improvement recommendations

### Workflow Optimization
- **Reduced Context Window Usage**: Efficiently provides database structure without consuming AI token context
- **Operation Chaining**: Enables complex multi-step operations with persistent context
- **Intelligent Defaults**: Suggests appropriate actions based on database structure and common patterns
- **Progressive Disclosure**: Reveals database complexity progressively as needed by the AI agent

## 🚀 Installation

### Prerequisites

- Go 1.18 or later
- Supported databases:
  - MySQL
  - PostgreSQL
  - (Additional databases in roadmap)
- Docker (optional, for containerized deployment)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/FreePeak/db-mcp-server.git
cd db-mcp-server

# Copy and configure environment variables
cp .env.example .env
# Edit .env with your configuration

# Option 1: Build and run locally with SSE transport (default)
make build
./mcp-server

# Option 2: Build and run with STDIO transport
make build
./mcp-server -t stdio

# Option 3: Using Docker
docker build -t db-mcp-server .
docker run -p 9090:9090 db-mcp-server

# Option 4: Using Docker Compose (with MySQL)
docker-compose up -d
```

### Transport Modes

The server supports two transport modes:

1. **SSE (Server-Sent Events)** - Default mode for browser and HTTP clients
   ```bash
   ./mcp-server -t sse
   ```

2. **STDIO (Standard Input/Output)** - For command-line tools and integrations
   ```bash
   ./mcp-server -t stdio
   ```
   
For STDIO mode, see the [examples directory](./examples) for usage examples.

### Docker

```bash
# Build the Docker image
docker build -t db-mcp-server .

# Run the container
docker run -p 9090:9090 db-mcp-server

# Run with custom configuration
docker run -p 8080:8080 \
  -e SERVER_PORT=8080 \
  -e LOG_LEVEL=debug \
  -e DB_TYPE=mysql \
  -e DB_HOST=my-database-server \
  db-mcp-server
  
# Run with Docker Compose (includes MySQL database)
docker-compose up -d
```

## 🔧 Configuration

DB MCP Server can be configured via environment variables or a `.env` file:

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | Server port | `9092` |
| `TRANSPORT_MODE` | Transport mode (stdio, sse) | `stdio` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `debug` |
| `DB_TYPE` | Database type (mysql, postgres) | `mysql` |
| `DB_HOST` | Database host | `localhost` |
| `DB_PORT` | Database port | `3306` |
| `DB_USER` | Database username | `iamrevisto` |
| `DB_PASSWORD` | Database password | `password` |
| `DB_NAME` | Database name | `revisto` |
| `DB_ROOT_PASSWORD` | Database root password (for container setup) | `root_password` |

See `.env.example` for more configuration options.

## 📖 Usage

### Integrating with Cursor Edit and AI Agents

DB MCP Server creates a powerful bridge between your databases and AI assistants in Cursor Edit, enabling AI-driven database operations with full context awareness. Configure your Cursor settings in `.cursor/mcp.json`:

```json
{
    "mcpServers": {
        "db-mcp-server": {
            "url": "http://localhost:9090/sse"
        }
    }
}
```

To leverage AI-powered database operations:

1. Configure and start the DB MCP Server using one of the installation methods above
2. Add the configuration to your Cursor settings
3. Open Cursor and navigate to a SQL or code file
4. The AI assistant now has access to your database schema, relationships, and capabilities
5. Ask the AI to generate, explain, or optimize database queries with full schema awareness
6. Execute AI-generated queries directly from Cursor

The MCP Server enhances AI assistant capabilities with:

- Complete database schema understanding
- Relationship-aware query generation
- Intelligent query optimization recommendations
- Error prevention through constraint awareness
- Performance metrics for better suggestions
- Context persistence across multiple operations

### Example AI Interactions

```
# Ask the AI for schema information
"What tables are in the database and how are they related?"

# Request query generation with context
"Create a query to find all orders from customers in California with items over $100"

# Get optimization suggestions
"How can I optimize this query that's taking too long to execute?"

# Request complex data operations
"Help me create a transaction that updates inventory levels when an order is placed"
```

## 📚 Documentation

### DB MCP Protocol for AI Integration

The server implements the DB MCP protocol with methods specifically designed to enhance AI agent capabilities:

- **initialize**: Sets up the session, transmits schema context, and returns server capabilities
- **tools/list**: Enables AI agents to discover available database tools dynamically
- **tools/call**: Allows AI to execute database tools with full context
- **editor/context**: Updates the server with editor context for better AI awareness
- **schema/explore**: Provides AI with detailed database structure information
- **cancel**: Cancels an in-progress operation

For full protocol documentation, visit the [MCP Specification](https://github.com/microsoft/mcp) and our database-specific extensions for AI integration.

### Tool System

The DB MCP Server includes a powerful AI-aware tool system that provides large language models and AI assistants with a structured way to discover and invoke database tools. Each tool has:

- A unique name discoverable by AI
- A comprehensive description that AI can understand
- A JSON Schema for input validation and AI parameter generation
- A structured output format that AI can parse and reason about
- A handler function that executes the tool's logic with context awareness

This structure enables AI agents to intelligently select, parameterize, and invoke the right database operations without requiring hard-coded knowledge of your specific database schema.

### Built-in Tools for AI Integration

The server includes AI-optimized database tools that provide rich context and capabilities:

| Tool | Description | AI Benefits |
|------|-------------|------------|
| `dbQuery` | Executes read-only SQL queries with parameterized inputs | Enables AI to retrieve data with full schema knowledge |
| `dbExecute` | Performs data modification operations (INSERT, UPDATE, DELETE) | Allows AI to safely modify data with constraint awareness |
| `dbTransaction` | Manages SQL transactions with commit and rollback support | Supports AI in creating complex multi-step operations |
| `dbSchema` | Auto-discovers database structure and relationships | Provides AI with complete schema context for reasoning |
| `dbQueryBuilder` | Visual SQL query construction with syntax validation | Helps AI create syntactically correct queries |
| `dbPerformanceAnalyzer` | Identifies slow queries and provides optimization suggestions | Enables AI to suggest performance improvements |
| `showConnectedDatabases` | Shows information about all connected databases | Enables AI to understand available database connections and their status |

### Multiple Database Support

DB MCP Server supports connecting to multiple databases simultaneously, allowing AI agents to work across different database systems in a unified way. Each database connection is identified by a unique ID that can be referenced when using database tools.

#### Configuring Multiple Databases
In your .env file
```
# Multi-Database Configuration
DB_CONFIG_FILE=config/databases.json
```
Configure multiple database connections in your `db-mcp-server/config/databases.json` file or environment variables:

```
# Multiple Database Configuration
{
  "connections": [
    {
      "id": "mysql1",
      "type": "mysql",
      "host": "localhost",
      "port": 13306,
      "user": "user1",
      "password": "password1",
      "name": "db1"
    },
    {
      "id": "mysql2",
      "type": "mysql",
      "host": "localhost",
      "port": 13307,
      "user": "user3",
      "password": "password3",
      "name": "db3"
    },
    {
      "id": "postgres1",
      "type": "postgres",
      "host": "localhost",
      "port": 15432,
      "user": "user2",
      "password": "password2",
      "name": "db2"
    }
  ]
} 
```

#### Viewing Connected Databases

Use the `showConnectedDatabases` tool to see all connected databases with their status and connection information:

```json
// Get information about all connected databases
{
  "name": "showConnectedDatabases"
}
```

Example response:

```json
[
  {
    "id": "mysql1",
    "type": "mysql",
    "host": "localhost",
    "port": 3306,
    "database": "db1",
    "status": "connected",
    "latency": "1.2ms"
  },
  {
    "id": "postgres1",
    "type": "postgres",
    "host": "localhost",
    "port": 5432,
    "database": "db2",
    "status": "connected",
    "latency": "0.8ms"
  }
]
```

#### Specifying Database for Operations

When using database tools, you must specify which database to use with the `database` parameter:

```json
// Query a specific database by ID
{
  "name": "dbQuery",
  "arguments": {
    "database": "postgres1",
    "query": "SELECT * FROM users LIMIT 10"
  }
}

// Execute statement on a specific database
{
  "name": "dbExecute",
  "arguments": {
    "database": "mysql2",
    "statement": "UPDATE products SET stock = stock - 1 WHERE id = 5"
  }
}

// Get schema from a specific database
{
  "name": "dbSchema",
  "arguments": {
    "database": "mysql1",
    "component": "tables"
  }
}
```

> **Note**: Always use `database` as the parameter name when specifying which database to use. This is required for all database operation tools.

If your configuration has only one database connection, you must still provide the database ID that matches the ID in your configuration.

### Database Schema Explorer Tool

The MCP Server includes an AI-aware Database Schema Explorer tool (`dbSchema`) that provides AI models with complete database structural knowledge:

```json
// Get all tables in the database - enables AI to understand available data entities
{
  "name": "dbSchema",
  "arguments": {
    "database": "mysql1",
    "component": "tables"
  }
}

// Get columns for a specific table - gives AI detailed field information
{
  "name": "dbSchema",
  "arguments": {
    "database": "postgres1",
    "component": "columns",
    "table": "users"
  }
}

// Get relationships for a specific table or all relationships - helps AI understand data connections
{
  "name": "dbSchema",
  "arguments": {
    "database": "mysql1",
    "component": "relationships",
    "table": "orders"
  }
}

// Get the full database schema - provides AI with comprehensive structural context
{
  "name": "dbSchema",
  "arguments": {
    "database": "postgres1",
    "component": "full"
  }
}
```

The Schema Explorer supports both MySQL and PostgreSQL databases, automatically adapting to your configured database type and providing AI with the appropriate contextual information.

### Visual Query Builder Tool

The MCP Server includes a powerful Visual Query Builder tool (`dbQueryBuilder`) that helps you construct SQL queries with syntax validation:

```json
// Validate a SQL query for syntax errors
{
  "name": "dbQueryBuilder",
  "arguments": {
    "database": "mysql1",
    "action": "validate",
    "query": "SELECT * FROM users WHERE status = 'active'"
  }
}

// Build a SQL query from components
{
  "name": "dbQueryBuilder",
  "arguments": {
    "database": "postgres1",
    "action": "build",
    "components": {
      "select": ["id", "name", "email"],
      "from": "users",
      "where": [
        {
          "column": "status",
          "operator": "=",
          "value": "active"
        }
      ],
      "orderBy": [
        {
          "column": "name",
          "direction": "ASC"
        }
      ],
      "limit": 10
    }
  }
}

// Analyze a SQL query for potential issues and performance
{
  "name": "dbQueryBuilder",
  "arguments": {
    "database": "mysql1",
    "action": "analyze",
    "query": "SELECT u.*, o.* FROM users u JOIN orders o ON u.id = o.user_id WHERE u.status = 'active' AND o.created_at > '2023-01-01'"
  }
}
```

Example response from a query build operation:

```json
{
  "query": "SELECT id, name, email FROM users WHERE status = 'active' ORDER BY name ASC LIMIT 10",
  "components": {
    "select": ["id", "name", "email"],
    "from": "users",
    "where": [{
      "column": "status",
      "operator": "=",
      "value": "active"
    }],
    "orderBy": [{
      "column": "name",
      "direction": "ASC"
    }],
    "limit": 10
  },
  "validation": {
    "valid": true,
    "query": "SELECT id, name, email FROM users WHERE status = 'active' ORDER BY name ASC LIMIT 10"
  }
}
```

The Query Builder supports:
- SELECT statements with multiple columns
- JOIN operations (inner, left, right, full)
- WHERE conditions with various operators
- GROUP BY and HAVING clauses
- ORDER BY with sorting direction
- LIMIT and OFFSET for pagination
- Syntax validation and error suggestions
- Query complexity analysis

### Performance Analyzer Tool

The MCP Server includes a powerful Performance Analyzer tool (`dbPerformanceAnalyzer`) that identifies slow queries and provides optimization suggestions:

```json
// Get slow queries that exceed the configured threshold
{
  "name": "dbPerformanceAnalyzer",
  "arguments": {
    "database": "mysql1",
    "action": "getSlowQueries",
    "limit": 5
  }
}

// Get metrics for all tracked queries sorted by average duration
{
  "name": "dbPerformanceAnalyzer",
  "arguments": {
    "database": "postgres1",
    "action": "getMetrics",
    "limit": 10
  }
}

// Analyze a specific query for optimization opportunities
{
  "name": "dbPerformanceAnalyzer",
  "arguments": {
    "database": "mysql1",
    "action": "analyzeQuery",
    "query": "SELECT * FROM orders JOIN users ON orders.user_id = users.id WHERE orders.status = 'pending'"
  }
}

// Reset all collected performance metrics
{
  "name": "dbPerformanceAnalyzer",
  "arguments": {
    "database": "postgres1",
    "action": "reset"
  }
}

// Set the threshold for identifying slow queries (in milliseconds)
{
  "name": "dbPerformanceAnalyzer",
  "arguments": {
    "database": "mysql1",
    "action": "setThreshold",
    "threshold": 300
  }
}
```

Example response from a performance analysis:

```json
{
  "query": "SELECT * FROM orders JOIN users ON orders.user_id = users.id WHERE orders.status = 'pending'",
  "suggestions": [
    "Avoid using SELECT * - specify only the columns you need",
    "Verify that ORDER BY columns are properly indexed",
    "Consider adding appropriate indexes for frequently queried columns"
  ]
}
```

Example response from getting slow queries:

```json
{
  "metrics": [
    {
      "query": "SELECT * FROM large_table WHERE status = ?",
      "count": 15,
      "totalDuration": "2.5s",
      "minDuration": "120ms",
      "maxDuration": "750ms",
      "avgDuration": "166ms",
      "lastExecuted": "2025-06-15T14:23:45Z"
    },
    {
      "query": "SELECT order_id, SUM(amount) FROM order_items GROUP BY order_id",
      "count": 8,
      "totalDuration": "1.2s",
      "minDuration": "110ms",
      "maxDuration": "580ms",
      "avgDuration": "150ms",
      "lastExecuted": "2025-06-15T14:20:12Z"
    }
  ],
  "count": 2,
  "threshold": "100ms"
}
```

The Performance Analyzer automatically tracks all query executions and provides:
- Identification of slow-performing queries
- Query execution metrics (count, min, max, average durations)
- Pattern-based query analysis
- Optimization suggestions
- Performance trend monitoring
- Configurable slow query thresholds

### Database Transactions Tool

For operations that require transaction support, use the `dbTransaction` tool:

```json
// Begin a transaction
{
  "name": "dbTransaction",
  "arguments": {
    "database": "mysql1",
    "action": "begin",
    "readOnly": false
  }
}

// Execute a statement within the transaction
{
  "name": "dbTransaction",
  "arguments": {
    "database": "mysql1",
    "action": "execute",
    "transactionId": "tx-1684785421293", // ID returned from the begin operation
    "statement": "INSERT INTO orders (customer_id, amount) VALUES (?, ?)",
    "params": ["123", "450.00"]
  }
}

// Commit the transaction
{
  "name": "dbTransaction",
  "arguments": {
    "database": "mysql1",
    "action": "commit",
    "transactionId": "tx-1684785421293"
  }
}

// Rollback the transaction (in case of errors)
{
  "name": "dbTransaction",
  "arguments": {
    "database": "mysql1",
    "action": "rollback",
    "transactionId": "tx-1684785421293"
  }
}
```

### Editor Integration

The server includes support for editor-specific features through the `editor/context` method, enabling tools to be aware of:

- Current SQL file
- Selected query
- Cursor position
- Open database connections
- Database structure

## 🗺️ Roadmap

We're committed to expanding DB MCP Server's AI integration capabilities. Here's our planned development roadmap:

### Q2 2025
- ✅ **AI-Aware Schema Explorer** - Auto-discover database structure and relationships for AI context
- ✅ **Context-Aware Query Builder** - AI-driven SQL query construction with syntax validation
- ✅ **Performance Analyzer with AI Insights** - Identify optimization opportunities with AI recommendations

### Q3 2025
- **AI-Powered Data Visualization** - Create charts and graphs from query results with AI suggestions
- **AI-Driven Model Generator** - Auto-generate code models from database tables using AI patterns
- **Multi-DB Support Expansion with Cross-DB AI Reasoning** - Add support with AI that understands:
  - **MongoDB** - Document-oriented schema for AI reasoning
  - **Redis** - Key-value pattern recognition for AI
  - **SQLite** - Lightweight database understanding

### Q4 2025
- **AI-Assisted Migration Manager** - Version-controlled schema changes with AI recommendations
- **Intelligent Access Control** - AI-aware permissions for database operations
- **Context-Enriched Query History** - Track queries with execution metrics for AI learning
- **Additional Database Integrations with AI Context**:
  - **Cassandra** - Distributed schema understanding
  - **Elasticsearch** - Search-optimized AI interactions
  - **DynamoDB** - NoSQL reasoning capabilities
  - **Oracle** - Enterprise schema comprehension

### Future Vision
- **Complete Database Coverage with Unified AI Context** - Support for all major databases with consistent AI interface
- **AI-Assisted Query Optimization** - Smart recommendations using machine learning
- **Cross-Database AI Operations** - Unified interface for heterogeneous database environments
- **Real-Time Collaborative AI** - Multi-user AI assistance for collaborative database work
- **AI-Powered Plugin System** - Community-driven extension marketplace with AI discovery

## 🤝 Contributing

Contributions are welcome! Here's how you can help:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b new-feature`
3. **Commit** your changes: `git commit -am 'Add new feature'` 
4. **Push** to the branch: `git push origin new-feature`
5. **Submit** a pull request

Please make sure your code follows our coding standards and includes appropriate tests.

## 📝 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 📧 Support & Contact

- For questions or issues, email [mnhatlinh.doan@gmail.com](mailto:mnhatlinh.doan@gmail.com)
- Open an issue directly: [Issue Tracker](https://github.com/FreePeak/db-mcp-server/issues)
- If DB MCP Server helps your work, please consider supporting:

<p align="">
<a href="https://www.buymeacoffee.com/linhdmn">
<img src="https://img.buymeacoffee.com/button-api/?text=Support DB MCP Server&emoji=☕&slug=linhdmn&button_colour=FFDD00&font_colour=000000&font_family=Cookie&outline_colour=000000&coffee_colour=ffffff" 
alt="Buy Me A Coffee"/>
</a>
</p>