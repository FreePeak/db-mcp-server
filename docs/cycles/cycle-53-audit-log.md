# Cycle 53 — JSONL Audit Trail (Bytebase governance parity)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `DB_MCP_AUDIT_LOG=/path/audit.jsonl` gives operators a persistent record of every statement executed through the server — the last unclaimed differentiator from the competitive baseline (Bytebase's audit governance), sized for an MCP process: one append-only file, one JSONL line per execution.
- Record shape: `{ts, op, database, statement, duration_ms, error}`; statements capped at 10k chars with an explicit marker so pathological payloads can't balloon the file.
- Coverage: `query`, `execute`, and all transaction actions (`tx_begin`/`tx_execute`/`tx_commit`/`tx_rollback`) via a deferred hook on each use-case method (named returns keep the diff to two lines per method).
- **Rejected writes are audited too**: attempts against read-only databases never reach the engine but are exactly what an operator needs to see — locked in by test.
- Env-only by design (audit destination belongs to deployment config, not per-database JSON); lazy first-use pickup means startup ordering doesn't matter. Best-effort writes never fail a query.

## Verification
- Five unit tests: entry shape for query/statement/failure paths, concurrent-write integrity (500 goroutine records → 500 intact lines — any mutex gap shows up as a corrupt line the JSON parse catches), truncation cap, disabled no-op, read-only rejection auditing.
- Full suite + live smoke green.

## Design Notes
- A recursive `sync.Once.Do` inside the lazy initializer would have deadlocked (documented Go behavior) — caught during self-review before commit.
