# Cycle 01 — Read-Only Bypass Fix + max_rows Guardrail

**Status:** ✅ Shipped · **Artifacts:** PR #85, commit `5ed77ef`

## Research Findings
- `ExecuteQuery` (behind `query_*` tools) performed zero read-only checks; issue #41's guard only covered `ExecuteStatement`. Agents could run INSERT/DELETE/DDL through the "query" tool on `read_only: true` databases.
- No row-limit support anywhere (grep: 0 matches) — a large `SELECT *` floods the agent context window. DBHub's headline guardrail.

## Shipped
- `internal/usecase/sql_guard.go`: statement classifier that strips comments, quoted strings/identifiers, backticks, and PostgreSQL dollar-quoting; scans for mutating keywords anywhere (catches data-modifying CTEs and stacked statements); default-deny for unrecognized leading keywords.
- `ExecuteQuery` refuses writes on read-only databases before touching the driver.
- `max_rows` per-database config option (all engines): results truncate with an explicit `[Truncated]` notice + shown count. Default unlimited for backward compatibility.
- Wiring: `pkg/db.Config.MaxRows`, `manager.DatabaseConnectionConfig.max_rows`, domain interface `MaxRows()`, repository adapter.

## Verification
- 36-case classifier table tests (comments, escapes, dollar-quoting, CTEs, stacked statements).
- Read-only bypass regression tests across 5 write vectors.
- Row-limit unit tests + end-to-end JSON→manager→real-SQLite test.
- go vet / golangci-lint / full suite green.

## Fed Forward
Read-only enforcement is textual only → cycle 03 adds engine-level enforcement.
