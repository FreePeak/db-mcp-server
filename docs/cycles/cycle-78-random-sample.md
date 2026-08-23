# Cycle 78 — Random Sampling (sample_rows)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Eyeballing a big table meant hand-writing engine-specific random
  ordering — `random()` (Postgres/SQLite), `RAND()` (MySQL),
  `DBMS_RANDOM.VALUE` (Oracle) — easy to get wrong from memory.

## Shipped

- `internal/usecase/query_sample.go`:
  - `randomOrderBy(dbType)`: engine map with a RANDOM() default.
  - `ExecuteQuerySample(ctx, dbID, query, params, n)` — wraps the
    statement as a subquery with the engine's random ORDER BY LIMIT n;
    read-only enforcement and SELECT/WITH admission as elsewhere;
    oversized samples return everything available; repo type lookup
    failure degrades to the default ordering.
- Query tool: `sample_rows` number param routed via capability interface
  `sampleQueryUseCase`, ahead of pagination.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestExecuteQuerySample`: exactly 10 of 100 rows returned, no
    out-of-range ids; oversized sample returns all 3 matching rows.
- Two compile fixes en route (nonexistent repo method → helper with
  fallback; missing logger import).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for sample_rows.
- Post-merge: verify npm v1.12.0 + docker tags published.
