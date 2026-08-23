# Cycle 121 — Timeout-Guardrail Audit (performance action=timeout_guardrails)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Per-call MCP deadlines (cycle 105) protect one client, but if the
  engine itself has no statement_timeout / idle-in-transaction ceiling
  / max_execution_time, a runaway query from ANY client runs until
  completion. Nothing read those settings. Confirmed absent.

## Shipped

- `internal/usecase/timeout_guardrails.go`:
  `CheckTimeoutGuardrails(ctx, dbID)` — Postgres reads pg_settings for
  statement_timeout + idle_in_transaction_session_timeout; MySQL reads
  @@max_execution_time. Zeros render "UNPROTECTED (no limit — any
  client's runaway query runs until completion)"; nonzero values render
  their millisecond ceiling; verdict names the unprotected count.
  SQLite errors "not available".
- Performance tool: new action `timeout_guardrails` (both per-db and
  unified constructors) served via capability interface
  `guardrailUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestTimeoutGuardrailsCatalog`: pg hits statement_timeout +
    idle_in_transaction_session_timeout; mysql hits
    max_execution_time; sqlite none.
  - `TestCheckTimeoutGuardrails_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=timeout_guardrails.
- Post-merge: verify npm v1.12.0 + docker tags published.
