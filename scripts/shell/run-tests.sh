#!/bin/bash

# Comprehensive test runner for all databases

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

function usage {
    echo "Usage: $0 [command]"
    echo "Commands:"
    echo "  unit            - Run unit tests only (no database required)"
    echo "  integration     - Run integration tests for all databases"
    echo "  oracle          - Run Oracle-specific tests only"
    echo "  regression      - Run regression tests across all databases"
    echo "  all             - Run all tests (unit + integration + regression)"
    echo "  coverage        - Run tests with coverage report"
    echo "  help            - Show this help message"
}

function run_unit_tests {
    echo -e "${BLUE}=== Running Unit Tests ===${NC}"
    go test -short -v ./...
    echo -e "${GREEN}Unit tests completed!${NC}\n"
}

function run_integration_tests {
    echo -e "${BLUE}=== Running Integration Tests ===${NC}"
    
    # Check if databases are running
    echo -e "${YELLOW}Checking database availability...${NC}"
    
    # Start test databases if not running
    if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting test databases...${NC}"
        docker-compose -f docker-compose.test.yml up -d
        sleep 10
    fi
    
    # Start Oracle if not running
    if ! docker-compose -f docker-compose.oracle.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting Oracle test environment...${NC}"
        ./oracle-test.sh start
    fi
    
    echo -e "${YELLOW}Running integration tests...${NC}"
    MYSQL_TEST_HOST=localhost \
    POSTGRES_TEST_HOST=localhost \
    ORACLE_TEST_HOST=localhost \
    go test -v ./pkg/db -run TestOracle
    
    MYSQL_TEST_HOST=localhost \
    POSTGRES_TEST_HOST=localhost \
    ORACLE_TEST_HOST=localhost \
    go test -v ./pkg/dbtools -run TestOracleIntegration
    
    echo -e "${GREEN}Integration tests completed!${NC}\n"
}

function run_oracle_tests {
    echo -e "${BLUE}=== Running Oracle Tests ===${NC}"
    
    # Ensure Oracle is running
    if ! docker-compose -f docker-compose.oracle.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting Oracle test environment...${NC}"
        ./oracle-test.sh start
    fi
    
    echo -e "${YELLOW}Running Oracle tests...${NC}"
    ORACLE_TEST_HOST=localhost go test -v ./pkg/db -run TestOracle
    ORACLE_TEST_HOST=localhost go test -v ./pkg/dbtools -run TestOracle
    
    echo -e "${GREEN}Oracle tests completed!${NC}\n"
}

function run_regression_tests {
    echo -e "${BLUE}=== Running Regression Tests ===${NC}"
    
    # Ensure all databases are running
    if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting test databases...${NC}"
        docker-compose -f docker-compose.test.yml up -d
        sleep 10
    fi
    
    if ! docker-compose -f docker-compose.oracle.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting Oracle test environment...${NC}"
        ./oracle-test.sh start
    fi
    
    echo -e "${YELLOW}Running regression tests...${NC}"
    MYSQL_TEST_HOST=localhost \
    POSTGRES_TEST_HOST=localhost \
    ORACLE_TEST_HOST=localhost \
    go test -v ./pkg/db -run TestRegressionAllDatabases
    
    echo -e "${YELLOW}Running connection pooling tests...${NC}"
    MYSQL_TEST_HOST=localhost \
    POSTGRES_TEST_HOST=localhost \
    ORACLE_TEST_HOST=localhost \
    go test -v ./pkg/db -run TestConnectionPooling
    
    echo -e "${GREEN}Regression tests completed!${NC}\n"
}

function run_all_tests {
    echo -e "${BLUE}=== Running All Tests ===${NC}\n"
    
    run_unit_tests
    run_integration_tests
    run_regression_tests
    
    echo -e "${GREEN}All tests completed successfully!${NC}"
}

function run_with_coverage {
    echo -e "${BLUE}=== Running Tests with Coverage ===${NC}"
    
    # Ensure all databases are running
    if ! docker-compose -f docker-compose.test.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting test databases...${NC}"
        docker-compose -f docker-compose.test.yml up -d
        sleep 10
    fi
    
    if ! docker-compose -f docker-compose.oracle.yml ps | grep -q "Up"; then
        echo -e "${YELLOW}Starting Oracle test environment...${NC}"
        ./oracle-test.sh start
    fi
    
    echo -e "${YELLOW}Running tests with coverage...${NC}"
    MYSQL_TEST_HOST=localhost \
    POSTGRES_TEST_HOST=localhost \
    ORACLE_TEST_HOST=localhost \
    go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
    
    echo -e "${YELLOW}Generating coverage report...${NC}"
    go tool cover -html=coverage.out -o coverage.html
    
    echo -e "${GREEN}Coverage report generated: coverage.html${NC}"
    echo -e "${YELLOW}Opening coverage report in browser...${NC}"
    
    # Try to open in browser
    if command -v xdg-open > /dev/null; then
        xdg-open coverage.html
    elif command -v open > /dev/null; then
        open coverage.html
    else
        echo -e "${YELLOW}Please open coverage.html manually in your browser${NC}"
    fi
}

# Main script
case "$1" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    oracle)
        run_oracle_tests
        ;;
    regression)
        run_regression_tests
        ;;
    all)
        run_all_tests
        ;;
    coverage)
        run_with_coverage
        ;;
    help)
        usage
        ;;
    *)
        usage
        exit 1
        ;;
esac

exit 0
