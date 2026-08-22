# Cycle 46 — Auto-LIMIT Injection (Server-Side Bound)

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Deferred twice for parser-risk, unblocked by the paren-depth scanner proven in cycles 32/35. The insight that dissolves the risk: a *subquery's* LIMIT never bounds the outer result set, so "no top-level LIMIT" is the correct and sufficient injection condition.
- Value: max_rows truncates client-side after the engine already materialized everything; injected LIMIT lets engines stop early — real cost savings on large tables.

## Shipped
- `applyAutoLimit(query, limit)`: appends `LIMIT n` to SELECT/WITH statements lacking a top-level LIMIT; respects existing limits; handles trailing semicolons; literals stripped before detection ("LIMIT" in strings ignored); non-SELECT statements untouched.
- `hasTopLevelLimit`: depth-aware LIMIT detection distinguishing outer from subquery clauses.
- Wired into ExecuteQuery + ExecuteQueryMasked via `autoLimitedQuery`: fires only when `db.MaxRows() > 0` and engine ≠ Oracle (different syntax); introspection failures leave statements untouched.

## Verification
- TDD RED-first: 8-case injection matrix (plain, existing limit, subquery limit still injects, ORDER BY append, semicolon strip, WHERE append, non-SELECT noop, zero-limit noop) plus top-level vs subquery vs string-literal LIMIT classification, and an end-to-end 50-row SQLite test proving exactly 10 rows arrive with no client truncation needed.
- Test-contract fix during GREEN: e2e originally asserted truncation notices — contradicts success (injected LIMIT means no overflow). Corrected to assert exact-count arrival.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Oracle support once ROW NUM wrapping is justified by demand.
- Per-database opt-out (`auto_limit: false`) if any workload needs raw scans.
