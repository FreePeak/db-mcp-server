#!/bin/bash
# Token-efficiency benchmark (backlog #2): measures the actual tools/list
# payload each registration mode puts on the wire, since that JSON is what
# an MCP client's context window pays for on every session start.
#
# Methodology is deliberately honest about its limits: token counts are
# estimated at 4 characters/token, a common approximation — treat deltas
# between modes as the signal, not absolute values. SQLite databases are
# generated so no external engine influences the schema descriptions.
#
# Usage: scripts/token-benchmark.sh [db-counts...]   (default: 1 3 10)
set -euo pipefail

cd "$(dirname "$0")/.."
COUNTS=("$@")
[ ${#COUNTS[@]} -eq 0 ] && COUNTS=(1 3 10)

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
BIN="$WORK/db-mcp-server"
go build -o "$BIN" ./cmd/server

measure() { # $1=config path, $2=extra flags
    local resp
    resp="$({
        printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench","version":"1.0"}}}'
        printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
        printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
        sleep 1
    } | CONFIG_PATH="$1" "$BIN" -t stdio -c "$1" $2 2>/dev/null | grep '"id":2')"
    # Extract the tools array from the JSON-RPC envelope and report raw size.
    local bytes
    bytes="$(printf '%s' "$resp" | wc -c | tr -d ' ')"
    echo "$bytes"
}

printf '%-10s %-12s %10s %12s\n' "databases" "mode" "json-bytes" "~tokens(4c/t)"
for n in "${COUNTS[@]}"; do
    cfg="$WORK/config_$n.json"
    {
        echo '{"connections":['
        for i in $(seq 1 "$n"); do
            db="$WORK/bench_$i.db"
            [ -f "$db" ] || sqlite3 "$db" "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT); CREATE INDEX idx_users_name ON users(name);"
            [ "$i" -gt 1 ] && echo ','
            printf '{"id":"bench%s","type":"sqlite","database_path":"%s"}' "$i" "$db"
        done
        echo ']}'
    } > "$cfg"

    per_db="$(measure "$cfg" "")"
    unified="$(measure "$cfg" "-unified-tools")"
    printf '%-10s %-12s %10s %12s\n' "$n" "per-db" "$per_db" "$((per_db / 4))"
    printf '%-10s %-12s %10s %12s\n' "" "unified" "$unified" "$((unified / 4))"
done
