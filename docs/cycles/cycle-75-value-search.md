# Cycle 75 — Cross-Table Value Search

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- filter_tables finds tables by *name*; nothing could answer "which table
  holds this email/UUID?" — the discovery step before writing any query.
- Surveyed competitor MCPs (Supabase, postgres-mcp): no equivalent. Also
  the natural completion of the profiling family (69).

## Shipped

- `internal/usecase/value_search.go`: `SearchValues(ctx, dbID, needle)` —
  per table: one OR'd COUNT over textual columns, then per-column probes
  for tables with hits. LIKE wildcards in the needle escaped (`\%`, `\_`,
  `\\`) so literals match literally; unreadable tables degrade to a
  "skipped" note; clean "No matches" report when absent.
- filter_tables tool: optional `value` param switches from name filtering
  to content search via capability interface `valueSearchUseCase`.

## Verification

- TDD RED first (build fail), then GREEN. Real bug found and fixed:
  - Per-column probe queries ran while the outer COUNT rows were still
    open — SQLite-style single-connection pools can't run nested queries,
    so every probe silently failed and the scan always reported "No
    matches". Fixed by closing outer rows before probing; caught by the
    RED test refusing to pass despite a manually-verified identical query.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for filter_tables value search.
- Post-merge: verify npm v1.12.0 + docker tags published.
