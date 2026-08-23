# Cycle 120 — Connection Saturation (performance action=connection_saturation)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- "FATAL: too many connections already" is an incident the tool
  surface could not see coming: health reports the server-side pool,
  not how close the engine is to its client ceiling. Confirmed absent;
  one catalog read per engine.

## Shipped

- `internal/usecase/connection_saturation.go`:
  `CheckConnectionSaturation(ctx, dbID)` — Postgres reads
  pg_stat_activity count vs current_setting('max_connections'); MySQL
  reads performance_schema Threads_connected vs max_connections.
  Renders usage with a verdict: ≥95% CRITICAL ("new clients will be
  refused"), ≥80% WARNING, else healthy. SQLite errors "not available".
  `toInt` handles both int64 and string driver values (MySQL status
  tables return strings).
- Performance tool: new action `connection_saturation` (both per-db and
  unified constructors) served via capability interface
  `saturationUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestConnectionSaturationCatalog`: pg hits pg_stat_activity +
    max_connections; mysql hits Threads_connected + max_connections;
    sqlite none.
  - `TestCheckConnectionSaturation_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=connection_saturation.
- Post-merge: verify npm v1.12.0 + docker tags published.
