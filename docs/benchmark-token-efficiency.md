# Token-Efficiency Benchmark: Unified vs Per-Database Tools

**Backlog #2** · Measured 2026-08-25 · Repeatable via `scripts/token-benchmark.sh`

## Why This Exists
DBHub's pitch is MCP token efficiency. This server's answer is `-unified-tools`: instead of one tool registration per tool-type × per-database, register each tool type once with a `database` parameter. What an MCP client actually pays for on every session start is the `tools/list` payload, so that is what we measure — not schema-source claims.

## Methodology
- Real stdio protocol session against the built server: `initialize` → `notifications/initialized` → `tools/list`; the id-2 response is captured whole.
- Databases are generated SQLite files with identical schema (users table + index), so descriptions don't vary by engine and no external service influences results.
- Token counts are estimated at **4 characters/token**, a common approximation. Treat *deltas between modes* as the signal, not absolute values. JSON escaping inflates byte counts slightly but identically across modes.
- Harness: `scripts/token-benchmark.sh [counts...]` — run it to reproduce.

## Results

| Databases | Per-db bytes | Per-db ~tokens | Unified bytes | Unified ~tokens | Savings |
|---|---|---|---|---|---|
| 1  | 4,042  | ~1,010 | 5,019 | ~1,254 | −24% (unified costs more) |
| 3  | 10,402 | ~2,600 | 5,307 | ~1,326 | 49% saved |
| 10 | 32,680 | ~8,170 | 6,333 | ~1,583 | 80% saved |

## Reading the Numbers
- **Per-db cost is linear**: every added database duplicates every tool type's full schema. At 10 databases that is ~8k tokens of context spent before any query runs.
- **Unified cost is near-flat**: the only growth is the comma-separated database list in each tool's description (~300 tokens from N=1→10 total, i.e. ~30/database).
- **Crossover is at ~2 databases**: with one database, unified pays slightly more for the `database` parameter and availability list. With two or more, unified wins and the gap widens without bound.
- Practical guidance: default single-database users can keep per-db mode (nicer tool ergonomics); anyone managing several databases should run `-unified-tools`, ideally with `--lazy-loading`.

## Honest Caveats
- The 4 chars/token estimate ignores tokenizer variance; relative savings hold under any consistent estimator because both payloads are similar JSON prose.
- DBHub publishes its own numbers under different schemas and workloads; this benchmark makes no attempt to normalize against them — it measures what this server puts on the wire so our own claim ("unified mode bounds context cost") is verifiable.
