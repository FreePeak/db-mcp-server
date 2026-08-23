# Cycle 83 — Stored Routine Listing (schema format=routines)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Stored functions/procedures were the last invisible behavior layer —
  business logic hiding in the engine. Completes the introspection
  family (tables → views → triggers → routines).

## Shipped

- `internal/usecase/routines.go`: `ListRoutines(ctx, dbID)` — per-engine
  catalog query (pg_proc excluding system schemas with function/procedure
  kinds, information_schema.ROUTINES, Oracle user_objects); renders name,
  type, and signature truncated at 200 chars. Engines without stored
  routines (SQLite) report a clean empty list rather than an error.
- Schema tool: `format: "routines"` routed via capability interface
  `routineListingUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestListRoutines`: SQLite (no routine support) reports the clean
    empty path.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=routines.
- Post-merge: verify npm v1.12.0 + docker tags published.
