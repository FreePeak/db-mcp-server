# Cycle 186 — temp_file_limit Audit (performance action=temp_file_limit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `temp_file_limit` caps per-session temporary files (sorts, hashes,
  materializations that spill to disk). The default `-1` means
  unlimited: one runaway query can fill the disk and take down every
  database on the host.
- A finite limit converts that blast radius into a per-query error
  instead of a host outage; the fix is reloadable (ALTER SYSTEM +
  pg_reload_conf()), so it's named in the warning.
- Zero renders as unlimited (Postgres treats non-positive as no
  cap); unparseable values render as the same warning rather than
  being guessed at.
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/temp_file_limit.go`:
  - `tempFileLimitProbe` — reads `current_setting('temp_file_limit')`
    in KB; postgres only.
  - `tempFileLimitVerdict` — pure classifier: ≤0 (unlimited) →
    WARNING naming the ALTER SYSTEM + pg_reload_conf() fix;
    bounded values render "" (audit adds explicit healthy line
    with humanBytes).
  - `AuditTempFileLimit` — runs the probe; unparseable values fall
    to the unlimited warning, never guessed at; unsupported engines
    get an explicit error.
- Performance tool: new action `temp_file_limit` (both per-db and
  unified constructors) served via capability interface
  `tempFileLimitUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe shape + engine gating; 10GB bounded → quiet; −1
  escalated with disk-fill mode + named fix; 0 renders "unlimited";
  explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=temp_file_limit.
- Post-merge: verify npm v1.12.0 + docker tags published.
