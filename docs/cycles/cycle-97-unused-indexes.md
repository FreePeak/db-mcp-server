# Cycle 97 — Unused Index Detection (query unused_indexes=true)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The index advisor (44) suggests missing indexes; nothing reported
  dead ones — every unused index taxes writes for zero read benefit.
  Together they complete index hygiene.

## Shipped

- `internal/usecase/unused_indexes.go`: `ListUnusedIndexes(ctx, dbID,
  minScans)` — Postgres: pg_stat_user_indexes filtered below threshold,
  unique indexes excluded, largest-first; MySQL: sys.schema_unused_indexes;
  engines without usage stats get an explicit unsupported error (never
  fabricated output). Default threshold 100 scans.
- Query tool: `unused_indexes: true` + `min_scans` params routed via
  capability interface `indexUsageUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestUnusedIndexQueries`: per-engine SELECTs exist and the PG one
    parameterizes the threshold ($1).
  - `TestListUnusedIndexes_Unsupported`: SQLite errors mentioning
    "usage statistic", no fabricated rows.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for unused_indexes.
- Post-merge: verify npm v1.12.0 + docker tags published.
