# Cycle 54 — Oracle Auto-LIMIT (ROWNUM wrap)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Fed-forward thread from cycle 46: `autoLimitedQuery` hard-excluded Oracle
  because `LIMIT` is not valid Oracle syntax — every unbounded SELECT on an
  Oracle connection materialized fully server-side, defeating the whole point
  of max_rows.
- Competitor scan: Oracle's portable row-bound across all versions is the
  ROWNUM wrap (`SELECT * FROM (...) WHERE ROWNUM <= n`); `FETCH FIRST n ROWS
  ONLY` exists only on 12c+. The wrap also preserves WITH clauses and
  top-level ORDER BY, which appending cannot.

## Shipped

- `internal/usecase/auto_limit.go`: `applyOracleRowLimit` (ROWNUM wrap for
  SELECT/WITH statements) and `hasTopLevelOracleBound` (depth-0 scan for
  top-level `ROWNUM` or `FETCH`; literals stripped first, subquery bounds do
  not suppress the wrap).
- `internal/usecase/database_usecase.go`: `autoLimitedQuery` routes the
  Oracle branch through the wrap instead of returning unchanged.
- `README.md`: Auto-LIMIT section updated — Oracle now bounded via ROWNUM
  wrap; exclusion language removed.

## Verification

- TDD RED first (`applyOracleRowLimit` undefined → build fail), then GREEN.
- One self-inflicted test bug caught by the suite itself: the
  literal-`'ROWNUM'` query was initially placed in the "untouched" table
  while its comment said it must be wrapped. Instrumentation inside
  `applyOracleRowLimit` proved the implementation matched the documented
  intent; the test data was contradictory and was fixed, not the code.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.
- New tests: `TestApplyOracleRowLimit` (wrap/no-op/bound-detection matrix),
  `TestAutoLimitedQuery_OracleRoutesToRownumWrap` (usecase-level routing,
  plus a LIMIT-dialect regression case).

## Fed Forward

- Engine-aware rewrite estimates via table-size catalogs (remaining thread).
- Content-PII match threshold tuning if masking proves noisy.
- Post-merge: verify npm v1.12.0 + docker tags publish.
