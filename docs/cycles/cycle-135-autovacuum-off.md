# Cycle 135 — Autovacuum-Disabled Table Audit (performance action=autovacuum_disabled)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Someone setting autovacuum_enabled=false on a big table years ago
  means dead-tuple bloat and XID-wraparound risk accumulate in silence
  — the wraparound and maintenance audits measure the damage, but only
  the reloptions storage parameters reveal the tables where the janitor
  was fired. Confirmed absent.

## Shipped

- `internal/usecase/autovacuum_off.go`:
  `ListAutovacuumDisabled(ctx, dbID)` — scans pg_class.reloptions for
  user tables (relkind r/p, non-system schemas) with
  autovacuum_enabled=false, rendering each with its row estimate and a
  re-enable hint (ALTER TABLE … SET (autovacuum_enabled = true); then
  VACUUM (ANALYZE)). Clean result stated explicitly. Postgres-only;
  other engines error "not available".
- Performance tool: new action `autovacuum_disabled` (both per-db and
  unified constructors) served via capability interface
  `autovacuumOffUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestAutovacuumOffCatalog`: hits reloptions +
    autovacuum_enabled=false; mysql/sqlite "".
  - `TestListAutovacuumDisabled_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=autovacuum_disabled.
- Post-merge: verify npm v1.12.0 + docker tags published.
