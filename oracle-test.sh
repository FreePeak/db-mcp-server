#!/bin/bash

# Script to manage the Oracle test environment

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

function usage {
    echo "Usage: $0 [command]"
    echo "Commands:"
    echo "  start       - Start the Oracle test environment"
    echo "  stop        - Stop the Oracle test environment"
    echo "  status      - Check the status of the Oracle test environment"
    echo "  logs        - View the logs of the Oracle test environment"
    echo "  restart     - Restart the Oracle test environment"
    echo "  cleanup     - Stop and remove the containers, networks, and volumes"
    echo "  test        - Run Oracle integration tests"
    echo "  help        - Show this help message"
}

function start {
    echo -e "${YELLOW}Starting Oracle test environment...${NC}"
    docker-compose -f docker-compose.oracle.yml up -d
    
    echo -e "${YELLOW}Waiting for Oracle to be ready...${NC}"
    echo -e "${YELLOW}This may take 1-2 minutes for first-time startup...${NC}"
    max_attempts=60
    attempt=0
    
    while ! docker-compose -f docker-compose.oracle.yml exec -T oracle-xe healthcheck.sh > /dev/null 2>&1; do
        attempt=$((attempt+1))
        if [ $attempt -ge $max_attempts ]; then
            echo -e "${RED}Failed to connect to Oracle after $max_attempts attempts.${NC}"
            echo -e "${YELLOW}Showing container logs:${NC}"
            docker-compose -f docker-compose.oracle.yml logs --tail=50 oracle-xe
            exit 1
        fi
        echo -e "${YELLOW}Waiting for Oracle to be ready (attempt $attempt/$max_attempts)...${NC}"
        sleep 2
    done
    
    echo -e "${GREEN}Oracle test environment is running!${NC}"
    echo -e "${YELLOW}Connection information:${NC}"
    echo "  Host: localhost"
    echo "  Port: 1521"
    echo "  User: testuser"
    echo "  Password: testpass"
    echo "  Service Name: TESTDB"
    echo "  Connection String: localhost:1521/TESTDB"
    echo ""
    echo -e "${YELLOW}You can connect to Oracle using:${NC}"
    echo "  sqlplus testuser/testpass@localhost:1521/TESTDB"
    echo ""
    echo -e "${YELLOW}Or using Docker exec:${NC}"
    echo "  docker exec -it db-mcp-oracle-test sqlplus testuser/testpass@TESTDB"
}

function stop {
    echo -e "${YELLOW}Stopping Oracle test environment...${NC}"
    docker-compose -f docker-compose.oracle.yml stop
    echo -e "${GREEN}Oracle test environment stopped.${NC}"
}

function status {
    echo -e "${YELLOW}Status of Oracle test environment:${NC}"
    docker-compose -f docker-compose.oracle.yml ps
}

function logs {
    echo -e "${YELLOW}Logs of Oracle test environment:${NC}"
    docker-compose -f docker-compose.oracle.yml logs "$@"
}

function restart {
    echo -e "${YELLOW}Restarting Oracle test environment...${NC}"
    docker-compose -f docker-compose.oracle.yml restart
    echo -e "${GREEN}Oracle test environment restarted.${NC}"
}

function cleanup {
    echo -e "${YELLOW}Cleaning up Oracle test environment...${NC}"
    docker-compose -f docker-compose.oracle.yml down -v
    echo -e "${GREEN}Oracle test environment cleaned up.${NC}"
}

function run_tests {
    echo -e "${YELLOW}Running Oracle integration tests...${NC}"
    
    # Check if Oracle is running
    if ! docker-compose -f docker-compose.oracle.yml ps | grep -q "Up"; then
        echo -e "${RED}Oracle test environment is not running.${NC}"
        echo -e "${YELLOW}Starting Oracle test environment first...${NC}"
        start
    fi
    
    echo -e "${YELLOW}Running Go tests...${NC}"
    ORACLE_TEST_HOST=localhost go test -v ./pkg/db -run TestOracle
    ORACLE_TEST_HOST=localhost go test -v ./pkg/dbtools -run TestOracle
    
    echo -e "${GREEN}Oracle integration tests completed!${NC}"
}

# Main script
case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    status)
        status
        ;;
    logs)
        shift
        logs "$@"
        ;;
    restart)
        restart
        ;;
    cleanup)
        cleanup
        ;;
    test)
        run_tests
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
