#!/bin/bash

# Set environment variables
export MCP_SERVER_NAME="multidb"
export CURSOR_EDITOR=1

# Get the absolute path of the script's directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR"

# Database config
CONFIG_FILE="config.json"

# Create logs directory if it doesn't exist
mkdir -p logs

# Generate a timestamp for the log filename
TIMESTAMP=$(date +"%Y%m%d-%H%M%S")
LOG_FILE="logs/cursor-mcp-$TIMESTAMP.log"

# Display startup message
echo "Starting DB MCP Server for Cursor..." >&2
echo "Config file: $CONFIG_FILE" >&2
echo "MCP Server Name: $MCP_SERVER_NAME" >&2
echo "Logs will be written to: $LOG_FILE" >&2

# Run the server in cursor mode with stdio transport
echo "Starting server..." >&2
exec ./server \
  -t stdio \
  -c "$CONFIG_FILE" \
  2> >(tee -a "$LOG_FILE" >&2) 