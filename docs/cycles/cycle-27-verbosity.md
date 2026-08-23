# Cycle 27 — Result Verbosity Modes (context-token compression)

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Fresh competitive scan (2026-08): cocaxcode/database-mcp's standout is verbosity modes with *measured* savings — `normal` (cell cap) −78%, `minimal` −96% vs raw psql; wide TEXT/JSON columns dominate agent token budgets. Neither DBHub nor MCP Toolbox offers cell-size control; our `max_rows` caps rows but not cell bytes.
- Migration-safety tooling (boringSQL/dryrun: offline lock analysis, rewrite detection) flagged as a future differentiator — Postgres-only, 25 stars, early.

## Shipped
- `ResultVerbosity` on the shared renderer: `full` (byte-identical legacy), `normal` (cells capped at 500 chars with explicit `…(+N chars)` markers, every row preserved), `minimal` (columns + honest total row count incl. max_rows overflow notice + truncated first-row preview).
- `verbosity` parameter on per-db and unified query tools; masking composes (mask → truncate order preserved); `ExecuteQueryVerbosity` use-case entry point.
- Schema contract guard extended to lock `verbosity` alongside `mask_pii`.

## Verification
- TDD RED-first: truncation marker + row survival, minimal compactness bound, full-mode byte-equality against legacy renderer, max_rows × minimal stacking, end-to-end SQLite with 3000-char cell.
- Test-contract corrections during GREEN: minimal previews must themselves be cell-truncated; minimal reports total rows honestly rather than reusing the "Truncated" wording.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Config-level default verbosity per database (`verbosity: "normal"`), mirroring mask_pii governance.
- Auto-LIMIT injection for read queries without LIMIT (cocaxcode parity).
