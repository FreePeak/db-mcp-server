# Cycle 129 — Partition Listing (schema format=partitions)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A large table may be partitioned, and nothing on the tool surface
  said so: an agent writing queries against the parent could not see
  child partitions, their bounds, or where rows live. Confirmed absent
  (zero "partition" matches in usecase/delivery).

## Shipped

- `internal/usecase/partitions.go`: `ListPartitions(ctx, dbID, table)`
  — Postgres reads pg_inherits joined to pg_class (child relname, row
  estimate, total size via pg_total_relation_size, parent bound via
  $1::regclass); MySQL reads information_schema.PARTITIONS scoped to
  DATABASE() with PARTITION_DESCRIPTION bounds. "Not partitioned" is
  stated explicitly rather than implied by an empty list; SQLite and
  other engines error "not available". Table name validated with
  isPlainIdentifier.
- Schema tool: `format: "partitions"` (requires `table` param) via
  capability interface `partitionUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestPartitionCatalog`: pg hits pg_inherits + $1 binding + relname;
    mysql hits information_schema.PARTITIONS + ? + DESCRIPTION; sqlite
    none.
  - `TestListPartitions_Unsupported`: explicit error.
  - Self-caught mid-cycle: Fprintf into a string (compile error) fixed
    before first green run.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=partitions.
- Post-merge: verify npm v1.12.0 + docker tags published.
