# Cycle 84 — CSV Bulk Import (execute csv_data=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Export landed in cycle 60/66; import was still hand-building INSERTs
  through separate calls. With atomic scripts (81) in place, CSV import
  composes cleanly and closes the last half of survey gap #1.

## Shipped

- `internal/usecase/csv_import.go`: `ImportCSV(ctx, dbID, table,
  csvContent)` — encoding/csv parse (first record sets the column count;
  ragged rows fail), header names validated as identifiers, values
  single-quote escaped, all inserts in one transaction with rollback
  naming the failing row; hard cap of 10,000 rows per import.
- Execute tool: `csv_data` + required `csv_table` params routed via
  capability interface `csvImportUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestImportCSV`: 2 plain rows inserted; quoted comma value stored
    verbatim ("Smith, John"); a ragged row fails the whole batch and the
    earlier insert is verified rolled back by re-query.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for CSV import.
- Post-merge: verify npm v1.12.0 + docker tags published.
