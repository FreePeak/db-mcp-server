# Cycle 196 — PostgreSQL Timeout-Guardrails Audit (performance action=timeout_guards)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `statement_timeout` and `idle_in_transaction_session_timeout` both
  default to `0` (unlimited). Without them, a runaway SELECT holds its
  locks until a human cancels it, and a forgotten open transaction
  blocks vacuum indefinitely — `long_transactions` (cycle 193) and
  `cancel_query` are reactive; this audit makes the gap visible so the
  engine can be made self-healing.
- Both fixes are runtime `ALTER SYSTEM`/`SET` — no restart required,
  so the warning names the exact statement with sensible values.
- Verdict ladder: either guard unset → one WARNING per missing guard
  naming its SET command; both set → quiet healthy line rendering the
  actual millisecond values.

## Shipped

- `internal/usecase/timeout_guards.go`:
  - `timeoutGuardsProbe` — reads both GUCs via `current_setting`;
    postgres/postgresql only.
  - `timeoutGuardsVerdict` — pure classifier per the ladder above.
  - `AuditTimeoutGuards` — runs the probe; unparseable or negative
    values are explicit errors, not silent zeros.
- Performance tool: new action `timeout_guards` (per-db + unified)
  via capability interface `timeoutGuardsUseCase`.
- Hardening evidence: full-tree `go test -race ./...` sweep at cycle
  start — zero data races across all packages.

## Verification

- TDD RED first, GREEN after implementation. Probe shape + engine
  gating; verdict ladder table (healthy quiet, each unset escalated
  with its SET fix, both unset → exactly two WARNINGs); explicit
  non-PG unsupported error for the audit path.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=timeout_guards.
- Post-merge: verify npm v1.12.0 + docker tags published.

## Loop Status

- Config-audit family additions now include timeout_guards. The GUC/
  variable sweep is approaching diminishing returns; next cycles
  should consider cross-cutting passes (docs consistency, regression
  sweeps) or user-driven priorities.
