#!/bin/bash
# Protocol-level smoke check for db-mcp-server.
#
# Drives a real stdio JSON-RPC session against the SQLite instance in
# config.test.json and asserts the outputs that unit harnesses cannot see:
# tracker wiring, guardrail visibility, and advisor actions. Run before
# tagging a release. Exits non-zero on the first failed assertion.
set -euo pipefail

cd "$(dirname "$0")/.."
CONFIG="${SMOKE_CONFIG:-config.test.json}"
DB_ID="${SMOKE_DB_ID:-test_sqlite}"

BIN="$(mktemp -t dbmcp-smoke.XXXXXX)"
trap 'rm -f "$BIN"' EXIT
go build -o "$BIN" ./cmd/server

run_session() {
    {
        printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0"}}}'
        printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
        printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"performance_${DB_ID}\",\"arguments\":{\"action\":\"suggest_indexes\",\"query\":\"SELECT id FROM users WHERE name = 'smoke_probe'\"}}}"
        for _ in 1 2 3; do
            printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":10,\"method\":\"tools/call\",\"params\":{\"name\":\"query_${DB_ID}\",\"arguments\":{\"query\":\"SELECT id FROM users WHERE name = 'smoke_probe'\"}}}"
        done
        sleep 1
        printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"performance_${DB_ID}\",\"arguments\":{\"action\":\"workload_suggestions\"}}}"
        printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"health_${DB_ID}\",\"arguments\":{}}}"
        sleep 1
    } | CONFIG_PATH="$CONFIG" "$BIN" -t stdio -c "$CONFIG" 2>/dev/null
}

OUT="$(run_session)"

fail() { echo "SMOKE FAIL: $1" >&2; exit 1; }
echo "$OUT" | grep -q "idx_users_name ON users (name)" \
    || fail "suggest_indexes did not propose idx_users_name"
echo "$OUT" | grep -q "ranked by estimated total time" \
    || fail "workload_suggestions missing duration weighting — tracker wiring broken?"
echo "$OUT" | grep -q "read_only:" \
    || fail "health output missing read_only guardrail"
echo "$OUT" | grep -Eq "statement_timeout_seconds: [0-9]+" \
    || fail "health output missing statement_timeout_seconds"

# The suggest_indexes and workload suggestions must agree on the same fix;
# cycle 40 switched weighting to engine-reported total time, so units are ms.
echo "$OUT" | grep -qE "serves [0-9]+ of [0-9]+ ms of engine time" \
    || fail "workload coverage annotation wrong"

echo "SMOKE OK: advisor, tracker wiring, and health guardrails all verified over live stdio protocol"
