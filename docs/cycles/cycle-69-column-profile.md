# Cycle 69 — Column Profiling (describe profile_column)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Agents exploring unfamiliar data had to write ad-hoc GROUP BY/COUNT
  queries to answer "is this column enum-like? how sparse is it?" — the
  exact facts that shape correct queries. One profiling call answers them.

## Shipped

- `internal/usecase/column_profile.go`: `ProfileColumn(ctx, dbID, table,
  column)` — rows, null count, distinct count, min/max, and top-3 values
  by frequency. Portable SQL only (`::text` cast tried first for Postgres,
  portable fallback); identifiers validated up front then quoted; NULL
  renders as a value in top_values.
- Describe tool: `profile_column` param switches from table metadata to
  the profile, via capability interface `columnProfilingUseCase`.

## Verification

- TDD RED first (undefined symbol → build fail), then GREEN:
  - `TestProfileColumn`: labeled fields (rows: 10, distinct: 2) plus top
    values free/pro; age's 5 NULLs reported; unknown table errors.
  - Test bug caught during RED: asserted an unlabeled "8" that never
    occurs — switched to labeled-line assertions.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for profile_column.
- Post-merge: verify npm v1.12.0 + docker tags published.
