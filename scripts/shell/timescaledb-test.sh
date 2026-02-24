#!/bin/bash

# Script to manage the TimescaleDB test environment

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

function usage {
    echo "Usage: $0 [command]"
    echo "Commands:"
    echo "  start       - Start the TimescaleDB test environment"
    echo "  stop        - Stop the TimescaleDB test environment"
    echo "  status      - Check the status of the TimescaleDB test environment"
    echo "  logs        - View the logs of the TimescaleDB test environment"
    echo "  restart     - Restart the TimescaleDB test environment"
    echo "  cleanup     - Stop and remove the containers, networks, and volumes"
    echo "  help        - Show this help message"
}

function start {
    echo -e "${YELLOW}Starting TimescaleDB test environment...${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml up -d
    
    echo -e "${YELLOW}Waiting for TimescaleDB to be ready...${NC}"
    max_attempts=30
    attempt=0
    
    while ! docker-compose -f docker-compose.timescaledb-test.yml exec timescaledb pg_isready -U timescale_user -d timescale_test > /dev/null 2>&1; do
        attempt=$((attempt+1))
        if [ $attempt -ge $max_attempts ]; then
            echo -e "${RED}Failed to connect to TimescaleDB after $max_attempts attempts.${NC}"
            exit 1
        fi
        echo -e "${YELLOW}Waiting for TimescaleDB to be ready (attempt $attempt/$max_attempts)...${NC}"
        sleep 2
    done
    
    echo -e "${GREEN}TimescaleDB test environment is running!${NC}"
    echo -e "${YELLOW}Connection information:${NC}"
    echo "  Host: localhost"
    echo "  Port: 15435"
    echo "  User: timescale_user"
    echo "  Password: timescale_password"
    echo "  Database: timescale_test"
    echo ""
    echo -e "${YELLOW}MCP Server:${NC}"
    echo "  URL: http://localhost:9093"
    echo ""
    echo -e "${YELLOW}You can access the TimescaleDB test environment using:${NC}"
    echo "  psql postgresql://timescale_user:timescale_password@localhost:15435/timescale_test"
    echo ""
    echo -e "${YELLOW}Available databases via MCP Server:${NC}"
    echo "  - timescaledb_test (admin access)"
    echo "  - timescaledb_readonly (read-only access)"
    echo "  - timescaledb_readwrite (read-write access)"
}

function stop {
    echo -e "${YELLOW}Stopping TimescaleDB test environment...${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml stop
    echo -e "${GREEN}TimescaleDB test environment stopped.${NC}"
}

function status {
    echo -e "${YELLOW}Status of TimescaleDB test environment:${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml ps
}

function logs {
    echo -e "${YELLOW}Logs of TimescaleDB test environment:${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml logs "$@"
}

function restart {
    echo -e "${YELLOW}Restarting TimescaleDB test environment...${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml restart
    echo -e "${GREEN}TimescaleDB test environment restarted.${NC}"
}

function cleanup {
    echo -e "${YELLOW}Cleaning up TimescaleDB test environment...${NC}"
    docker-compose -f docker-compose.timescaledb-test.yml down -v
    echo -e "${GREEN}TimescaleDB test environment cleaned up.${NC}"
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
    help)
        usage
        ;;
    *)
        usage
        exit 1
        ;;
esac

exit 0 