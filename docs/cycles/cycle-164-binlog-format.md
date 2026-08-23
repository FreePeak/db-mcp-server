# Cycle 164 — binlog_format Audit (performance action=binlog_format)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `binlog_format=STATEMENT` replication silently diverges on
  non-deterministic functions (NOW(), UUID(), LIMIT without ORDER BY);
  MIXED still routes many statement shapes through unsafe STATEMENT
  logging. ROW is the MySQL default since 5.7.7 and required by most
  managed platforms. Primary↔replica drift is invisible until a
  failover serves wrong data — one of the nastiest failure modes to
  discover. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/binlog_format.go`:
  - `binlogFormatQuery` — @@GLOBAL.binlog_format;
    mysql/mariadb only.
  - `binlogFormatVerdict` — pure classifier: STATEMENT → WARNING
    naming the non-deterministic-divergence risk with SET GLOBAL +
    replica-thread-restart fix; MIXED/other → WARNING with ROW fix;
    ROW → "" (audit adds the explicit clean line); empty/unreadable →
    verify note.
  - `AuditBinlogFormat` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `binlog_format` (both per-db and
  unified constructors) served via capability interface
  `binlogFormatUseCase`.

## Verification

- TDD RED first (build fail), then one GREEN fix: the STATEMENT
  message rendered "DIVERGE" in caps while the assertion checked
  lowercase "diverge" — normalized the message wording before GREEN.
  - `TestBinlogFormatProbe`: probe shape + engine gating.
  - `TestBinlogFormatVerdict`: ROW renders empty; STATEMENT escalated
    with divergence wording + SET GLOBAL fix; MIXED escalated; empty
    flagged unreadable.
  - `TestAuditBinlogFormat_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=binlog_format.
- Post-merge: verify npm v1.12.0 + docker tags published.
