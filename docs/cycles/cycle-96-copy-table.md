# Cycle 96 — Cross-Database Table Copy (execute copy_table=)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Survey gap #2's last remnant: seeding staging from prod or backfilling
  an analytics table meant export → paste → execute by hand. The pieces
  existed (export 66, scripts 81, multi-repo access); composing them
  server-side closes the workflow class.

## Shipped

- `internal/usecase/copy_table.go`: `CopyTable(ctx, srcDB, dstDB, table)`
  — column list from the source catalog (order-stable, explicit), rows
  read and scanned as driver values, inserted in 500-row parameterized
  batches inside **one destination transaction**; any batch failure rolls
  back everything and names the target. Target table must already exist;
  same-db copies rejected; cross-engine fidelity best-effort (documented).
- Execute tool: `copy_table` + `from_db` params routed via capability
  interface `tableCopyUseCase`.

## Verification

- TDD RED first, then GREEN:
  - `TestCopyTable`: 3 rows land on the destination; re-copy fails with
    an error naming `dst.items`; missing target table fails clearly.
  - Routing case added to `TestCapabilityRouting` (`copy:src->db1:items`).
- Duplicate helper folded into existing `quoteIdentList`.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for copy_table.
- Post-merge: verify npm v1.12.0 + docker tags published.
