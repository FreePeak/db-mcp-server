# Cycle 20 — Statement Timeout Enforcement

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Competitor scan: DBHub ships max_rows/timeouts as core safety; Postgres MCP Pro's restricted mode is read-only + time caps. We had read-only (cycle 03) and max_rows (cycle 01) but zero statement timeouts — an agent firing a runaway query at production held its connection until the client gave up.
- In-code audit found the gap was worse than missing: `pkg/db` has carried a `QueryTimeout` config (default 30s, JSON `query_timeout`, even applied as MySQL `readTimeout`) since an unmerged earlier attempt — advertised but never enforced. No execution path wrapped its context.
- The right choke point is the repository adapter (`DatabaseAdapter.Query/Exec/Begin`): every tool path — query, execute, transactions, explain, describe, schema, health — crosses it.

## Shipped
- `DatabaseAdapter.timeout(ctx)` wraps every execution context with the database's configured timeout. Cancellation propagates engine-side on drivers that support it (PostgreSQL, MySQL); other engines are bounded at this layer.
- Row-lifetime correctness: for `Query`, the cancel func travels with the result set (`RowsAdapter.cancel`) and fires on `Close` — a deferred cancel would have killed streaming rows. `Exec` and `Begin` defer safely (`BeginTx` is the only context consumer in a transaction).
- Zero or absent config disables the cap; the optional-capability type assertion matches the existing `MaxRows()` pattern.

## Verification
- New `internal/repository/database_repository_test.go` (first tests in this package): fake driver records received contexts — deadline present with configured timeout (~5s/~3s bounds), absent when capability missing or set to 0, across Query/Exec/Begin.
- Full suite green across all packages; vet/gofmt clean.

## Fed Forward
- Surface the effective timeout in `health_<db_id>` output so operators can verify enforcement per database.
- Optional per-database env override (`QUERY_TIMEOUT_SECONDS`) for deployments that configure databases via environment rather than JSON.
