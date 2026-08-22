# Cycle 25 — Durable Masking Audit Sink

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- In-memory audit (cycle 24) evaporates on restart — unacceptable for governance trails that may need to answer "what did agents try to read?" after an incident.
- JSONL append-only files are the lowest-dependency durable format: streamable, greppable, rotation-friendly.

## Shipped
- `maskingAudit` optional append-mode JSONL sink: `EnableMaskingAuditFile(path)` / `CloseMaskingAuditFile()`; one event per line with full MaskingAuditEvent payload.
- Server flag `-masking-audit-log <path>` wires it at startup with deferred close and error logging; sink write failures are deliberately non-fatal (query serving never breaks for audit reasons).
- Re-enabling swaps sinks safely (previous handle closed with error propagation).

## Verification
- TDD RED-first: persisted-event parse-back (JSONL validity + field assertions), append mode preserving pre-existing content, default in-memory behavior untouched.
- errcheck strictness: this repo flags even blank assignments — replaced intentional ignores with real error propagation plus a single documented nolint for the best-effort write.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Log rotation for long-running 24/7 deployments if trails grow unbounded.
- Release: version bump to 1.12.0 queued for PR #87 merge.
