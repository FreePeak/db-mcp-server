# Cycle 21 — Guardrail Visibility + Env Timeout Override

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Planned to build DBHub's dual-layer guardrail (SQL classifier + engine enforcement) and found the classifier already exists: `internal/usecase/sql_guard.go` strips string/dollar-quoted literals, default-denies unknown leading verbs, and catches data-modifying CTEs and stacked statements. Dual-layer parity landed earlier than the backlog implied — backlog #1's premise ("Oracle needs engine enforcement") is softened by the fact the classifier layer already protects Oracle today.
- Real gap identified instead: guardrails are invisible. Cycle 20 made `QueryTimeout` actually enforced, but nothing in tool output shows which caps are active, and env-only deployments (no JSON config) had no way to set a timeout at all.

## Shipped
- `health_<db_id>` / unified `health` now reports active guardrails: `read_only`, `max_rows` (when > 0), and `statement_timeout_seconds` (when > 0). Operators verify enforcement instead of trusting config files.
- `QUERY_TIMEOUT_SECONDS` env override (`applyQueryTimeoutOverride`, internal/config/config.go): fills unset per-connection timeouts; JSON `query_timeout` keeps precedence; `-1` disables explicitly; invalid input warns without blocking startup.

## Verification
- `TestHealthCheck_GuardrailsVisible`: guarded fake asserts all three keys surface with correct values.
- Four subtests for the override helper: fills unset / keeps explicit / `-1` propagates / invalid and <-1 rejected with warning.
- Full suite green across all packages; vet/gofmt clean.

## Fed Forward
- Backlog #1 restated: Oracle engine-level read-only is now belt-and-suspenders (classifier already guards it); still worth landing when an Oracle container is available.
- Column masking (backlog #5) needs SELECT-target-to-table resolution — requires real SQL parsing or engine catalog help; scope it as its own multi-cycle effort before starting.
