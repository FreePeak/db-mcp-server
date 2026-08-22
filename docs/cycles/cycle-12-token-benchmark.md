# Cycle 12 — Token-Efficiency Benchmark (Measured)

**Status:** ✅ Shipped · **Artifacts:** `internal/delivery/mcp/tool_token_benchmark_test.go`, README numbers

## Research Findings
- DBHub's headline marketing number is "2 tools ≈ 1.4k tokens, 13-14x fewer than alternatives". We had no equivalent measurement, and the unified-mode README section made no quantitative claim.

## Shipped
- `TestToolTokenBenchmark`: marshals every registered tool definition (name + description + parameter schemas) in both modes across 1/3/10 connected databases and estimates tokens at the standard ~4 chars/token. Includes guardrails: unified surface must stay under 8k tokens; per-db must cost ≥4x unified at 10 databases.
- Measured results (this repo's real tool definitions):

| Databases | Unified | Per-database | Ratio |
|---|---|---|---|
| 1 | 1,087 tok | 799 tok | 0.7x |
| 3 | 1,127 tok | 2,397 tok | 2.1x |
| 10 | **1,271 tok** | **7,992 tok** | **6.3x** |

- README updated: honest crossover note (per-db slightly cheaper with exactly one database; unified wins from two onward and scales flat), plus fixed a pre-existing typo in that section and refreshed its stale tool list.

## Verification
- Benchmark passes; guardrail assertions lock the flat-scaling property so future tool additions cannot silently blow up the unified surface.

## Fed Forward
DBHub reaches ~1.4k with just two tools; our 8-tool unified set lands in the same range while carrying more capability — good position. Progressive disclosure (search_objects-style) remains the next lever if descriptions grow.
