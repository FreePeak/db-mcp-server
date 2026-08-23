# Cycle 125 — Sequential-Scan Workload Audit (performance action=seq_scan_heavy)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The static audits (fk_indexes, suggest_indexes) reason about shape;
  pg_stat_user_tables.seq_scan and MySQL's null-index rows in
  table_io_waits_summary_by_index_usage record what actually happened.
  Tables dominated by sequential scans are where missing indexes hurt
  for real. Confirmed absent.

## Shipped

- `internal/usecase/seq_scans.go`:
  - `seqScanQuery(dbType)` — Postgres reads pg_stat_user_tables
    (relname, seq_scan, idx_scan) hottest-first; MySQL aggregates
    table_io_waits_summary_by_index_usage per OBJECT_NAME with
    INDEX_NAME IS NULL as the full-scan proxy; SQLite "".
  - `seqScanVerdict(seq, idx)` — pure classifier: "no scans recorded",
    "indexing candidate" (seq ≥ 10 and seq > 3×idx), "index access
    dominates", or "mixed access".
  - `FindSeqScanHeavy(ctx, dbID)` — renders every tracked table's
    counters with its verdict plus a candidate count summary.
- Performance tool: new action `seq_scan_heavy` (both per-db and
  unified constructors) served via capability interface
  `seqScanUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestSeqScanCatalog`: pg hits pg_stat_user_tables + seq_scan;
    mysql hits table_io_waits_summary_by_index_usage + INDEX_NAME IS
    NULL; sqlite none.
  - `TestFindSeqScanHeavy_Unsupported`: explicit error.
  - `TestRenderSeqScanVerdict`: candidate flagged, healthy table not,
    cold table explicit.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=seq_scan_heavy.
- Post-merge: verify npm v1.12.0 + docker tags published.
