# Testing Guide for DB MCP Server

This document provides a comprehensive guide to testing the DB MCP Server, including Oracle database integration and regression tests.

## Quick Start

### Run All Tests
```bash
# Run unit tests (no database required)
make test-unit

# Run all tests including integration
make test-all

# Run tests with coverage report
make test-coverage
```

## Test Organization

### Unit Tests
Located throughout the codebase with `_test.go` suffix. These tests:
- Don't require database connections
- Run with `-short` flag
- Test individual functions and components in isolation

**Run unit tests:**
```bash
go test -short ./...
# or
make test-unit
```

### Integration Tests

#### All Databases
Tests actual database functionality across MySQL, PostgreSQL, SQLite, and Oracle.

**Setup:**
```bash
# Start all test databases
docker-compose -f docker-compose.test.yml up -d
./oracle-test.sh start
```

**Run tests:**
```bash
make test-integration
```

#### Oracle-Specific Tests

**File:** `pkg/dbtools/oracle_integration_test.go`

Tests Oracle-specific features:
- Query tool with Oracle functions (SYSDATE, SYSTIMESTAMP, DUAL)
- Execute operations (CREATE TABLE, INSERT, UPDATE, DELETE)
- Schema introspection (user_tables, user_tab_columns, user_constraints)
- Transaction management (commit/rollback)
- Performance analysis (EXPLAIN PLAN)
- Oracle sequences (NEXTVAL, CURRVAL)
- Data dictionary queries

**Setup:**
```bash
./oracle-test.sh start
```

**Run tests:**
```bash
./oracle-test.sh test
# or
make test-oracle
# or manually
ORACLE_TEST_HOST=localhost go test -v ./pkg/db -run TestOracle
ORACLE_TEST_HOST=localhost go test -v ./pkg/dbtools -run TestOracleIntegration
```

**Cleanup:**
```bash
./oracle-test.sh stop      # Stop Oracle container
./oracle-test.sh cleanup   # Remove container and volumes
```

### Regression Tests

**File:** `pkg/db/regression_test.go`

Comprehensive tests ensuring backward compatibility across all database types:

1. **TestRegressionAllDatabases** - Tests basic operations across all DBs:
   - Connection and ping
   - Basic queries
   - Execute operations (CREATE, INSERT, UPDATE, DELETE)
   - Transaction support (commit/rollback)
   - Data type handling

2. **TestConnectionPooling** - Tests connection pooling:
   - Concurrent connections
   - Connection limits
   - Connection reuse

**Setup:**
```bash
docker-compose -f docker-compose.test.yml up -d
./oracle-test.sh start
```

**Run tests:**
```bash
make test-regression
# or
MYSQL_TEST_HOST=localhost \
POSTGRES_TEST_HOST=localhost \
ORACLE_TEST_HOST=localhost \
go test -v ./pkg/db -run TestRegression
```

## Test Scripts

### `run-tests.sh`
Comprehensive test runner with multiple modes:

```bash
./run-tests.sh unit           # Unit tests only
./run-tests.sh integration    # Integration tests
./run-tests.sh oracle          # Oracle-specific tests
./run-tests.sh regression      # Regression tests
./run-tests.sh all             # All tests
./run-tests.sh coverage        # Tests with coverage report
```

### `oracle-test.sh`
Oracle test environment manager:

```bash
./oracle-test.sh start         # Start Oracle container
./oracle-test.sh status        # Check status
./oracle-test.sh logs          # View logs
./oracle-test.sh test          # Run Oracle tests
./oracle-test.sh stop          # Stop container
./oracle-test.sh cleanup       # Remove container and volumes
```

## Continuous Integration

### GitHub Actions Workflow

**File:** `.github/workflows/go.yml`

Three jobs run on every PR:

1. **Build & Test** - Unit tests and build verification
   - Go 1.22
   - Unit tests with `-short` flag
   - Build verification

2. **Integration Tests** - Full database testing
   - Services: MySQL 8.0, PostgreSQL 15, Oracle XE 21
   - Oracle schema initialization
   - Integration tests
   - Regression tests
   - Connection pooling tests
   - Code coverage reporting

3. **Lint** - Code quality checks
   - golangci-lint v2.0.0
   - Timeout: 5 minutes

### Environment Variables

For integration and regression tests, set these environment variables:

```bash
export MYSQL_TEST_HOST=localhost
export POSTGRES_TEST_HOST=localhost
export ORACLE_TEST_HOST=localhost
```

## Test Configuration Files

### `config.oracle-test.json`
Example Oracle test configuration:
```json
{
  "connections": [
    {
      "id": "oracle_test",
      "type": "oracle",
      "host": "localhost",
      "port": 1521,
      "service_name": "TESTDB",
      "user": "testuser",
      "password": "testpass",
      "query_timeout": 30,
      "max_open_conns": 10,
      "max_idle_conns": 5,
      "conn_max_lifetime_seconds": 1800
    }
  ]
}
```

## Best Practices

### Before Submitting a PR

1. Run unit tests: `make test-unit`
2. Run integration tests for affected databases
3. Run regression tests if changing core functionality
4. Ensure linting passes: `make lint`
5. Check code coverage: `make test-coverage`

### Adding New Tests

1. **Unit Tests**: Add next to the code being tested with `_test.go` suffix
2. **Integration Tests**: Add to appropriate test file, mark with `if testing.Short() { t.Skip() }`
3. **Regression Tests**: Add to `pkg/db/regression_test.go` or `pkg/dbtools/oracle_integration_test.go`

### Test Naming Conventions

- `TestUnitFunctionName` - Unit tests
- `TestIntegrationFeature` - Integration tests
- `TestRegressionScenario` - Regression tests

### Skipping Tests

Integration tests automatically skip when databases are unavailable:

```go
func TestMyIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    db, err := connectToDatabase()
    if err != nil {
        t.Skipf("Skipping test: database not available (%v)", err)
        return
    }
    // ... test code
}
```

## Troubleshooting

### Oracle Container Won't Start

```bash
# Check container logs
./oracle-test.sh logs

# Full cleanup and restart
./oracle-test.sh cleanup
./oracle-test.sh start
```

### Tests Fail with Connection Timeout

```bash
# Increase wait time in oracle-test.sh
# Or manually wait for Oracle to be ready
docker exec -it db-mcp-oracle-test healthcheck.sh
```

### Coverage Report Not Generated

```bash
# Ensure all databases are running
docker-compose -f docker-compose.test.yml up -d
./oracle-test.sh start

# Run coverage with verbose output
./run-tests.sh coverage
```

## Coverage Goals

- **Unit Tests**: > 80% coverage
- **Integration Tests**: Test all MCP tool functions
- **Regression Tests**: Cover all database types and core operations

## Resources

- [GitHub Actions Workflow](.github/workflows/go.yml)
- [Oracle Test Script](oracle-test.sh)
- [Test Runner Script](run-tests.sh)
- [Makefile Test Targets](Makefile)
- [Oracle Integration Tests](pkg/dbtools/oracle_integration_test.go)
- [Regression Tests](pkg/db/regression_test.go)
