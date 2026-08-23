# Cycle 183 — back_log Audit (performance action=back_log)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `back_log` is MySQL's TCP listen backlog for new connections.
  Bursts beyond it are refused by the kernel *before* authentication
  ever runs, so a load spike surfaces as connect-failed storms while
  `max_connections` still shows headroom — misleading during
  incidents.
- The setting is read-only at runtime: raising it requires my.cnf
  plus restart. Discovering that mid-incident is painful; the audit
  names the fix up front.
- `-1` (MySQL 8 default) means autosized from max_connections and is
  healthy by definition.
- Escalation ladder: 0/unparseable → unreadable; -1 → autosized,
  no warning; <64 → WARNING naming the pre-auth refusal mode and
  restart-required fix; ≥64 → "" (audit adds explicit healthy line).
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/back_log.go`:
  - `backLogProbe` — pairs `@@GLOBAL.back_log` with
    `@@GLOBAL.max_connections`; mysql/mariadb only.
  - `backLogVerdict` — pure classifier per the ladder
    (`backLogFloor = 64`).
  - `AuditBackLog` — runs the probe; unparseable counters render as
    unreadable, never guessed at; the `-1` sentinel renders its
    autosized line explicitly; unsupported engines get an explicit
    error.
- Performance tool: new action `back_log` (both per-db and unified
  constructors) served via capability interface `backLogUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe pairs setting with max_connections + engine gating;
  500 quiet; -1 renders "autosized" without WARNING; 10 escalated
  with pre-auth loss mode + restart fix; 0 unreadable; explicit
  non-MySQL unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=back_log.
- Post-merge: verify npm v1.12.0 + docker tags published.
