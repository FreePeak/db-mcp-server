#!/bin/bash

# This wrapper script ensures proper STDIO handling for the MCP server in Docker

# Export required environment variables
export MCP_DISABLE_LOGGING=true
export DISABLE_LOGGING=true
export TRANSPORT_MODE=stdio

# Create a log directory
mkdir -p /tmp/logs

# Run the server with proper redirection
# All stdout goes to the MCP proxy, while stderr goes to a file
exec /app/multidb-linux -t stdio 2>/tmp/logs/server.log 