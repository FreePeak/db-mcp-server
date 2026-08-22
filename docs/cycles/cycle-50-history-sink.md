# Cycle 50 — Durable Query-History Sink

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- First seeded thread from LOOP_STATE.md: masking audits got a durable trail (cycle 25) but query history (cycle 48) died with the process. Post-incident questions like "what did the agent run before the crash?" need persistence.

## Shipped
- `queryHistoryStore` optional append-mode JSONL sink: `EnableQueryHistoryFile(path)` / `CloseQueryHistoryFile()`; every recorded execution also lands in the file.
- Fail-fast at configuration: invalid sink paths error at enable-time with clear messages; once running, write failures never break execution.
- Re-enable after close supported (rotation scenario).

## Verification
- TDD RED-first: JSONL parse-back of a persisted INSERT entry; directory-as-sink fails fast at open with execution unaffected; round-trip re-enable.
- Test-contract correction during GREEN: original draft expected open to succeed and writes to fail — fail-fast at config is strictly better and now locked in.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Server flag/env for history sink (`-query-history-log`) mirroring masking-audit wiring.
- Rotation policy for long-running 24/7 deployments.
