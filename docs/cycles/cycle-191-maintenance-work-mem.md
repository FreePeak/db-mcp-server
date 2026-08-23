# Cycle 191 — maintenance_work_mem Audit (performance action=maintenance_work_mem)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The 64MB default makes `VACUUM`, `CREATE INDEX`, and `ANALYZE`
  spill to temp disk on large tables — index builds and dead-tuple
  scans stretch maintenance windows once the working set exceeds the
  budget.
- Unlike `work_mem` this budget is per maintenance operation, not per
  sort node / connection, so raising it is low-risk on typical
  servers — the warning says so explicitly.
- Ladder: ≥256MB → quiet (audit adds explicit healthy line);
  <256MB → WARNING naming the affected operations + the reloadable
  fix (`ALTER SYSTEM SET ... = '256MB'` + pg_reload_conf).
- Reused existing `parsePrettySize` (shared_buffers) for setting
  parsing and `humanBytes` for rendering; no new parsers.

## Shipped

- `internal/usecase/maintenance_work_mem.go`:
  - `maintenanceWorkMemProbe` — reads
    `current_setting('maintenance_work_mem')`; postgres only.
  - `maintenanceWorkMemVerdict` — pure classifier on parsed bytes:
    ≥256MB → ""; <256MB → escalated with ops + fix named; ≤0 →
    unreadable note.
  - `AuditMaintenanceWorkMem` — runs the probe; unsupported engines
    get an explicit error.
- Performance tool: new action `maintenance_work_mem` (both per-db
  and unified constructors) served via capability interface
  `maintenanceWorkMemUseCase`.

## Verification

- TDD RED first, GREEN after implementation. One test-expectation
  correction: `humanBytes` renders `64.0 MB` (decimal), test updated
  to match the established renderer — production code unchanged.
- Tests: probe shape + engine gating; 256MB quiet; default-64MB
  escalation naming VACUUM/CREATE INDEX/fix; zero unreadable;
  explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=maintenance_work_mem.
- Post-merge: verify npm v1.12.0 + docker tags published.
