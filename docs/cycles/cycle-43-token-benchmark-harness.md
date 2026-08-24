# Cycle 43 — Token-Benchmark Harness (backlog #2 hardening)

**Status:** Shipped · **Artifacts:** `scripts/token-benchmark.sh`, `docs/benchmark-token-efficiency.md`

## Context
Cycle 12 measured token cost via a Go test over marshaled tool definitions, with scaling guardrails. What was missing: a way to re-measure the *actual wire payload* without writing code — the JSON-RPC `tools/list` response is what clients pay for, including envelope overhead the unit-level test never sees.

## Shipped
- `scripts/token-benchmark.sh`: reproducible end-to-end harness. Builds the server, generates N identical SQLite databases, runs real stdio sessions in both modes, captures the id-2 `tools/list` responses, reports bytes and ~tokens. Takes arbitrary database counts as arguments.
- `docs/benchmark-token-efficiency.md`: methodology (4 chars/token estimate, deltas-over-absolutes guidance), measured results, crossover analysis, and honest caveats — including that DBHub publishes numbers under different schemas/workloads and no normalization is attempted.
- README unified-tools section now links both.

## Refreshed Numbers (wire payload, N=1/3/10)
| Databases | Per-db ~tok | Unified ~tok | Savings |
|---|---|---|---|
| 1 | ~1,010 | ~1,254 | −24% |
| 3 | ~2,600 | ~1,326 | 49% |
| 10 | ~8,170 | ~1,583 | 80% |

Confirms cycle 12's shape exactly: per-db linear (~800 tok/database), unified near-flat (~30 tok/database of description growth), crossover at two databases. Wire numbers run slightly above the marshaled-def numbers because envelopes and JSON escaping are included.

## Backlog Impact
- Backlog #2 stays fully done; the measurement is now repeatable by anyone in one command.
- Backlog #6 marked done as stale: `RelationshipGraph` has rendered the whole database's FK relationships as Mermaid since the describe/FK work — "multi-table relationship graph remains" no longer described reality.

## Verification
- Harness output matches expectations at three counts; full suite green; smoke passes.
