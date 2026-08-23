# Cycle 98 — Long-Query Triage (query long_queries=N)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #5's remaining half: "the database feels slow" had no
  activity view — raw pg_stat_activity SQL is blocked on read-only
  configs. One param answers it.

## Shipped

- `internal/usecase/long_queries.go`: `ListLongQueries(ctx, dbID,
  minSeconds)` — Postgres: active sessions past threshold via
  pg_stat_activity (own session excluded); MySQL: information_schema
  .processlist Query commands over the threshold; engines without an
  activity catalog error explicitly. Default triage threshold 30s.
- Query tool: `long_queries: N` param routed via capability interface
  `longQueryUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestLongQueryCatalogs`: per-engine SELECTs exist and both
    parameterize the age threshold ($1 / ?).
  - `TestListLongQueries_Unsupported`: SQLite errors mentioning
    "activity".
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for long_queries.
- Post-merge: verify npm v1.12.0 + docker tags published.
