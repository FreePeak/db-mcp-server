# Cycle 85 — Custom Type Listing (schema format=types)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Enum/composite types matter when generating migrations or code against
  Postgres; agents had no way to see them. Completes the introspection
  family alongside views (80), triggers (82), routines (83).

## Shipped

- `internal/usecase/types.go`: `ListCustomTypes(ctx, dbID)` — Postgres
  catalog query over pg_type (enum labels aggregated in sort order,
  composites noted); engines without the concept report a clean empty
  list.
- Schema tool: `format: "types"` routed via capability interface
  `customTypeListingUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestListCustomTypes`: SQLite reports the clean empty path.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=types.
- Post-merge: verify npm v1.12.0 + docker tags published.
