# Cycle 51 — Query-History Sink Server Wiring

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 50 built the sink; operators needed the switch. Masking-audit wiring (cycle 25) established the exact pattern: flag + env default + deferred close + error logging.

## Shipped
- `-query-history-log <path>` server flag with `DB_MCP_QUERY_HISTORY_LOG` env default; deferred close; startup log line. README flag table + env-defaults note updated.

## Verification
- Build + full suite (9 packages), vet, golangci-lint clean; flag path mirrors the already-tested masking-audit wiring. Zero Docker.

## Fed Forward
- Rotation policy for both JSONL trails on long-running deployments.
