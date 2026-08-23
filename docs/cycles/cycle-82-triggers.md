# Cycle 82 — Trigger Listing (schema format=triggers)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Agents couldn't see hidden behavior behind plain INSERTs/UPDATEs —
  audit writes and denormalization via triggers were invisible. Same
  engine-catalog pattern as views (80).

## Shipped

- `internal/usecase/triggers.go`: `ListTriggers(ctx, dbID)` — per-engine
  catalog query (pg_trigger excluding internal, information_schema.TRIGGERS,
  sqlite_master), rendering name, target table, and whitespace-collapsed
  definition truncated at 300 chars; unsupported engines error clearly.
- Schema tool: `format: "triggers"` routed via capability interface
  `triggerListingUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestListTriggers`: audit_insert listed on users with its INSERT body.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for format=triggers.
- Post-merge: verify npm v1.12.0 + docker tags published.
