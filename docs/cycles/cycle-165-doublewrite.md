# Cycle 165 — innodb_doublewrite Audit (performance action=doublewrite)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The doublewrite buffer is InnoDB's defense against torn pages: a
  crash mid-write can leave a page half-written inside the tablespace,
  which is silent data corruption that only surfaces when a backup is
  restored. It is sometimes disabled for benchmarks ("doublewrite
  costs ~5% writes") and forgotten. ON (the default) is healthy.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/doublewrite.go`:
  - `doublewriteQuery` — @@GLOBAL.innodb_doublewrite;
    mysql/mariadb only.
  - `doublewriteVerdict` — pure classifier: OFF → WARNING naming
    torn-page corruption with the my.cnf + restart fix (and the note
    that MySQL 8.0.30+ can SET GLOBAL it live); ON/1 → "" (audit adds
    the explicit clean line); empty/unreadable → verify note.
  - `AuditDoublewrite` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `doublewrite` (both per-db and unified
  constructors) served via capability interface `doublewriteUseCase`.

## Verification

- TDD RED first (build fail), then one GREEN fix: the OFF message
  said "can tear pages" while the assertion checked for "torn" —
  rewrote the wording to name torn-page protection explicitly before
  GREEN.
  - `TestDoublewriteProbe`: probe shape + engine gating.
  - `TestDoublewriteVerdict`: ON and "1" render empty; OFF escalated
    with torn-page wording + config/restart fix; empty flagged
    unreadable.
  - `TestAuditDoublewrite_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=doublewrite.
- Post-merge: verify npm v1.12.0 + docker tags published.
