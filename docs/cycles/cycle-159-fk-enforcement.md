# Cycle 159 — SQLite FK Enforcement Audit (performance action=fk_enforcement)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- SQLite parses FOREIGN KEY constraints but does not enforce them
  unless `PRAGMA foreign_keys=ON` is set on the connection — the
  default is OFF. With it off, writes of orphaned child rows succeed
  silently and referential integrity is only a promise in the schema.
  The setting is per-connection (cannot flip inside a transaction), so
  the audit covers the connections this server itself uses. Confirmed
  absent from the tool surface.

## Shipped

- `internal/usecase/fk_enforcement.go`:
  - `fkEnforcementQuery` — PRAGMA foreign_keys; sqlite/sqlite3 only.
  - `fkEnforcementVerdict` — pure classifier: OFF → WARNING naming
    silent orphan writes with the per-connection PRAGMA fix; ON → ""
    (audit adds the explicit clean line).
  - `AuditFKEnforcement` — runs the pragma against the live database,
    parses defensively, renders verdict or healthy line; unsupported
    engines get an explicit error.
- Performance tool: new action `fk_enforcement` (both per-db and
  unified constructors) served via capability interface
  `fkEnforcementUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestFKEnforcementProbe`: probe shape + engine gating.
  - `TestFKEnforcementVerdict`: ON renders empty; OFF escalated with
    "orphan" + PRAGMA fix.
  - `TestAuditFKEnforcement_EndToEnd`: audit runs against a real
    SQLite database via the standard test harness.
  - `TestAuditFKEnforcement_Unsupported`: explicit error for non-SQLite.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=fk_enforcement.
- Post-merge: verify npm v1.12.0 + docker tags published.
