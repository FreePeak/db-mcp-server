# Cycle 174 — binlog_row_image Audit (performance action=binlog_row_image)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With row-based replication (the modern MySQL default), the
  `binlog_row_image=FULL` setting logs complete before+after row
  images on every UPDATE/DELETE even when one column changed —
  extra disk, I/O, and replica network proportional to table width.
  MINIMAL logs only identifying columns plus what changed; NOBLOB
  skips unchanged BLOBs as a middle ground.
- Dynamic setting: SET GLOBAL binlog_row_image='MINIMAL' + my.cnf /
  SET PERSIST persistence. Caveat surfaced in the warning: flashback
  / audit tooling that reconstructs old rows requires FULL.
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/binlog_row_image.go`:
  - `binlogRowImageQuery` — @@GLOBAL probe; mysql/mariadb only.
  - `binlogRowImageVerdict` — pure classifier: FULL → WARNING
    naming before+after image overhead, the dynamic SET GLOBAL fix,
    and the flashback-tool caveat; MINIMAL/NOBLOB → "" (audit adds
    the explicit lean line); empty/unreadable → verify note.
  - `AuditBinlogRowImage` — runs the probe, renders verdict or
    healthy line; unsupported engines get an explicit error.
- Performance tool: new action `binlog_row_image` (both per-db and
  unified constructors) served via capability interface
  `binlogRowImageUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestBinlogRowImageProbe`: probe shape + engine gating.
  - `TestBinlogRowImageVerdict`: MINIMAL/NOBLOB render empty; FULL
    escalated naming the live fix and flashback caveat; empty
    flagged unreadable.
  - `TestAuditBinlogRowImage_Unsupported`: explicit non-MySQL error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=binlog_row_image.
- Post-merge: verify npm v1.12.0 + docker tags published.
