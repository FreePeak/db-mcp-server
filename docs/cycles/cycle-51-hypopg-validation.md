# Cycle 51 — Planner-Validated Index Suggestions via hypopg

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- New `validate_suggestions` performance action closes the biggest remaining gap to Postgres MCP Pro: heuristic suggestions are now **ground-truth validated against the planner** instead of "verify with EXPLAIN yourself".
- Mechanics: the same candidate builder as `suggest_indexes` (equality-first composites, capped at 3 columns; range/join singles) installs each proposal as a cost-free hypothetical index via `hypopg_create_index`, runs EXPLAIN on the original query, and reports ✓ USED / ✗ UNUSED per candidate from actual planner output. Hypothetical indexes vanish on reset — nothing is written.
- Engine-gated honestly: PostgreSQL/TimescaleDB only; other engines get an actionable refusal naming their manual path. Extension absence degrades to install guidance (best-effort `CREATE EXTENSION` first, then presence check).
- Live validation found and fixed a real bug: the composite leader-suppression guard fired even when no composite was emitted, silently dropping lone equality filters (`WHERE tenant_id = 3` produced zero candidates). Regression test added.

## Verification
- Live end-to-end against real PostgreSQL 15 + hypopg (installed into the compose container from PGDG): USED verdict for an unindexed equality filter; catalog leak check confirms nothing persists after validation.
- Unit tests lock candidate building (composite cap, join/range singles, lone-equality fix), structural result parsing, and non-PG refusal. Full suite green uncached; smoke green over live stdio.

## Design Notes
- CI's stock postgres:15 service lacks hypopg, so the live test skips there gracefully — same pattern as pg_stat_statements-dependent tests.
- The validator parses the standard row format structurally (dash-rule delimited data cells) rather than guessing name contents — hypopg names are opaque `<oid>…` strings.
