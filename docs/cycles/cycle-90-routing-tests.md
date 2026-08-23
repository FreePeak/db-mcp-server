# Cycle 90 — Delivery Routing Tests (capability wiring)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- 15 capability-interface routes shipped this session with only
  compile-level coverage; a misrouted param would fail silently at
  runtime. Wiring is the likeliest regression class.

## Shipped

- `internal/delivery/mcp/query_read_modes_test.go`: TestQueryToolRouting
  — databases= fan-out, sample_rows, page/page_size route to their
  capabilities; plain queries and single-database lists stay on
  ExecuteQuery.
- `internal/delivery/mcp/capability_routing_test.go`:
  TestCapabilityRouting — value search, related_key, duplicates_column,
  script, csv import, and all five schema formats (views/triggers/
  routines/types/ddl) each land on their usecase method.

## Verification

- Two test-authoring bugs fixed en route (stale stub state across cases;
  blank parameters needing explicit types) — no production changes.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- New routes should add a case here.
- Post-merge: verify npm v1.12.0 + docker tags published.
