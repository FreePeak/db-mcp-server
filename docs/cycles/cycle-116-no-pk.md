# Cycle 116 — No-Primary-Key Audit (schema format=no_pk)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- A table without a PRIMARY KEY breaks row-based and logical
  replication (engines need an identity column), makes rows
  unaddressable for targeted updates, and hides duplicates. Static
  check over the constraint catalog DescribeTable already returns.
  Confirmed absent.

## Shipped

- `internal/usecase/no_pk.go`: `FindTablesWithoutPK(ctx, dbID)` —
  walks every user table's constraints; flags those with no PRIMARY
  KEY row (composite PKs count as keyed). Renders the concrete risks
  per table plus a surrogate-key suggestion with an explicit carve-out
  for intentionally keyless tables (log append). Clean state names the
  scanned table count; SQLite-only databases still auditable since it
  reads DescribeTable metadata.
- Schema tool: `format: "no_pk"` via capability interface
  `noPKUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFindTablesWithoutPK`: keyless flagged; keyed and composite-
    PK tables silent; fully-keyed database renders "have a primary
    key" clean state.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=no_pk.
- Post-merge: verify npm v1.12.0 + docker tags published.
