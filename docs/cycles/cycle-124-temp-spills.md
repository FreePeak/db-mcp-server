# Cycle 124 — Temp-Spill Detection (performance action=temp_spills)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Sorts and hash joins exceeding the engine's memory budget spill to
  disk — the classic "queries feel slow but CPU is idle" signal of
  undersized work_mem / tmp_table_size. Nothing on the tool surface
  read pg_stat_database.temp_files/temp_bytes or MySQL's
  Created_tmp_disk_tables/Created_tmp_tables counters. Confirmed
  absent.

## Shipped

- `internal/usecase/temp_spills.go`: `CheckTempSpills(ctx, dbID)` —
  Postgres reads temp file count + pretty-printed bytes for the current
  database since stats reset; MySQL reads Created_tmp_disk_tables vs
  Created_tmp_tables and renders a spill ratio with a verdict
  (≥20% on-disk → consider raising tmp_table_size/max_heap_table_size,
  else healthy). Zero-state messages explicit ("No queries have spilled
  to disk"). SQLite errors "not available". Reuses `toInt` from cycle
  120 for MySQL string status values.
- Performance tool: new action `temp_spills` (both per-db and unified
  constructors) served via capability interface `tempSpillUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestTempSpillCatalog`: pg hits pg_stat_database + temp_files;
    mysql hits both Created_tmp counters; sqlite none.
  - `TestCheckTempSpills_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=temp_spills.
- Post-merge: verify npm v1.12.0 + docker tags published.
