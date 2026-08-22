# Cycle 73 — Per-Query Timeout (timeout_ms)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Agents running speculative or recursive queries had no self-service
  bound short of operator-side statement_timeout. A context deadline per
  call is the portable answer — no engine-specific SQL needed.

## Shipped

- `internal/usecase/query_timeout.go`: `ExecuteQueryWithTimeout(ctx,
  dbID, query, params, timeoutMs)` — wraps ExecuteQuery in a child
  context; timeoutMs ≤ 0 degrades to the plain path; deadline errors are
  surfaced as "query exceeded Nms timeout" only when the parent context
  is still live (caller cancellation isn't mislabeled).
- Query tool: `timeout_ms` number param; when set, the masked path runs
  under a child deadline (masking + verbosity preserved), falling back to
  the plain timeout wrapper for non-masking providers.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestExecuteQueryWithTimeout`: infinite recursive CTE dies with a
    deadline error at 50ms; a normal query passes under 5000ms.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for timeout_ms.
- Post-merge: verify npm v1.12.0 + docker tags published.
