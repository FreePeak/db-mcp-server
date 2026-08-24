# Cycle 57 — TimescaleDB discovery tools: compression, retention, continuous aggregates

**Status:** ✅ Shipped
**Theme:** Close the remaining TimescaleDB truth gap — expose the four read-only
discovery operations as real tools (per-db + unified), route every catalog SELECT
through the read-only-safe `ExecuteQuery` path, and put all of them behind the
runtime `ensureTimescaleExtension` guard introduced in cycle 56.

## Objective

Cycles 55–56 wired `list_hypertables` end to end and added the extension guard,
but compression settings, retention policy inspection, and continuous aggregate
discovery were still unreachable: their handlers existed but no tool registered
them, and they executed pure SELECTs through `ExecuteStatement`, which a
`read_only` database correctly refuses.

## What shipped

1. **Four read-only tools registered** (`tool_registry.go`)
   - `timescaledb_compression_settings_<db>`
   - `timescaledb_retention_policy_<db>`
   - `timescaledb_list_continuous_aggregates_<db>`
   - `timescaledb_continuous_aggregate_info_<db>`
   - Registered inside the shared `registerTimescaleTools` closure so eager and
     lazy-loading modes behave identically, plus matching unified-mode
     registrations (`<tool>` with a `database` parameter).

2. **Unified tool creators** (`timescale_tool.go`) —
   `CreateUnifiedCompressionSettingsTool`, `CreateUnifiedRetentionPolicyTool`,
   `CreateUnifiedContinuousAggregateListTool`,
   `CreateUnifiedContinuousAggregateInfoTool`, mirroring the cycle-55/56
   hypertable pattern.

3. **Read-only-safe read paths** — `get_compression_settings`,
   `get_retention_policy`, `list_continuous_aggregates`, and
   `get_continuous_aggregate_info` now call `ExecuteQuery` instead of
   `ExecuteStatement`. Pure catalog SELECTs stay available on `read_only`
   databases; mutating operations (add/remove policy, compression toggles)
   intentionally keep `ExecuteTransaction`/`ExecuteStatement`.

4. **Extension guard everywhere** — each of the four handlers calls
   `ensureTimescaleExtension` after the postgres-type check, so a plain
   PostgreSQL database gets an actionable
   "enable it with CREATE EXTENSION timescaledb" error instead of a raw
   catalog-missing failure.

5. **Tests updated**
   - `compression_policy_test.go`, `retention_policy_test.go`: mocks moved from
     `ExecuteStatement` to the probe + `ExecuteQuery` pair; the empty-result
     get-retention test asserts the explicit "empty result means none is
     configured" note instead of a synthesized message.
   - `timescale_tools_test.go`: list/info aggregates tests gain probe mocks and
     rendered-row result shapes.
   - `timescale_tool_test.go`: unified creator coverage.

## Verification

- `go build ./...` clean.
- `go test ./... -short -count=1` green across all packages.
- `gofmt -l` / `go vet ./...` clean.
- Mock-contract tests lock in: probe → ExecuteQuery ordering, rendered-text
  result parsing, and read_only availability of the four discovery paths.

## Artifacts

- Commit: this cycle's commit on `main`.
- Follows: [cycle-55](cycle-55-timescale-truth-gap.md),
  [cycle-56](cycle-56-timescale-lazy-loading.md).
