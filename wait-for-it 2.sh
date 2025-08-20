#!/bin/sh
# wait-for-it.sh - Enhanced version for database connection checking
# Usage: wait-for-it.sh host port [timeout]

set -e

host="$1"
port="$2"
timeout="${3:-30}"

if [ -z "$host" ] || [ -z "$port" ]; then
  echo "Error: Host and port are required arguments"
  echo "Usage: wait-for-it.sh host port [timeout]"
  exit 1
fi

echo "Waiting for $host:$port to be available..."
start_time=$(date +%s)
end_time=$((start_time + timeout))

while true; do
  # Try to establish a TCP connection to the specified host and port
  if nc -z -w 1 "$host" "$port" 2>/dev/null; then
    echo "$host:$port is available"
    exit 0
  fi
  
  current_time=$(date +%s)
  remaining=$((end_time - current_time))
  
  if [ $current_time -gt $end_time ]; then
    echo "ERROR: Timeout waiting for $host:$port to be available after $timeout seconds"
    echo "Network diagnostics:"
    echo "Current container IP: $(hostname -I || echo 'Unknown')"
    echo "Attempting to ping $host:"
    ping -c 1 -W 1 "$host" || echo "Ping failed"
    echo "Attempting DNS lookup for $host:"
    nslookup "$host" || echo "DNS lookup failed"
    echo "Network interfaces:"
    ifconfig || ip addr show || echo "Network tools not available"
    exit 1
  fi
  
  echo "Waiting for $host:$port to be available... (${remaining}s timeout remaining)"
  sleep 1
done 