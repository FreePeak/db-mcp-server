# Cycle 197 — Combined Configuration Health Audit (performance action=health_audit)
**Status:** Shipped · **Branch:** hackathon (PR #87)
## Research
- The performance tool grew ~60 individual configuration audits
  (crash_safety, wal_mode, synchronous_commit, busy_timeout,
  wait_timeout, buffer_pool, …). An agent doing "is this database
  configured sanely?" must know which audits exist and call each one —
  N round trips for what is conceptually one question.
- This audit runs every registered config audit once, merges their
  verdicts into a single per-section report, and renders only the
  sections that carry findings plus an explicit all-clean summary.
  Individual actions remain available for targeted follow-up on any
  WARNING line.
## Shipped

- `internal/usecase/health_audit.go`:
  - `healthAuditSection` — one audit registered as (name, runner);
    failures degrade to `"<name>: unavailable (<err>)"` instead of
    failing the whole report.
  - `registerHealthAudits` — the registry: crash_safety, wal_mode,
    synchronous_commit, busy_timeout, track_io_timing, wait_timeout,
    buffer_pool, max_packet, table_cache. Engine-gated audits render
    their own unsupported message; MySQL-only ones are skipped
    silently on other engines.
  - `RunHealthAudit` — executes sections sequentially, prints
    `== name ==` headers with findings, or one clean summary line when
    nothing is at risk.
- Performance tool: new action `health_audit` (per-db + unified) via
  capability interface `healthAuditUseCase`.

## Verification

- TDD RED first, GREEN after implementation: mixed-engine fake repo
  proves section headers appear, warnings surface verbatim, and the
  clean path renders the explicit healthy summary.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

---
*Cycle protocol: TDD red→green, wire into tool surface, lint, docs,
commit, push to origin/hackathon.*
