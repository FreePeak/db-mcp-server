# Cycle 132 — Extension Listing (performance action=list_extensions)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- performance.go probes for pg_stat_statements specifically, but
  nothing shows the full picture — which extensions are installed
  (timescaledb? postgis?) and what is available but not yet installed.
  One catalog read answers "why does engine_slow_queries say
  unsupported?" before anyone guesses. Confirmed absent.

## Shipped

- `internal/usecase/extensions.go`: `ListExtensions(ctx, dbID)` — one
  UNION query over pg_extension (installed, with version) and
  pg_available_extensions minus already-installed (available), rendered
  as two sections with counts. Postgres-only; other engines error "not
  available".
- Performance tool: new action `list_extensions` (both per-db and
  unified constructors) served via capability interface
  `extensionUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestExtensionCatalog`: hits pg_extension + pg_available_extensions;
    mysql/sqlite "".
  - `TestListExtensions_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=list_extensions.
- Post-merge: verify npm v1.12.0 + docker tags published.
