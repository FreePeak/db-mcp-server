# Cycle 143 — Binary-Log Growth Audit (performance action=binlog_growth)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Binary logs make point-in-time recovery and replication possible —
  and, with binlog_expire_logs_seconds=0, silently fill the disk.
  Nothing on the tool surface reported their size or retention.
  Confirmed absent.

## Shipped

- `internal/usecase/binlogs.go`:
  - `binlogRetentionQuery` — reads @@GLOBAL.binlog_expire_logs_seconds;
    mysql/mariadb only.
  - `AuditBinaryLogs(ctx, dbID)` — probes retention, then SHOW BINARY
    LOGS to sum file sizes (generic column scan locating File_size by
    name, int64 or []byte); renders a single verdict line.
  - `binlogVerdict(files, expireSecs, totalBytes)` — pure classifier:
    zero files → logging disabled/just rotated; zero retention →
    WARNING naming the setting; else healthy with days-to-expiry.
- Performance tool: new action `binlog_growth` (both per-db and unified
  constructors) served via capability interface `binaryLogUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestBinlogRetentionQuery`: hits binlog_expire_logs_seconds;
    postgres/sqlite "".
  - `TestBinlogVerdict`: never-expire warning vs healthy escalation
    proven.
  - `TestAuditBinaryLogs_Unsupported`: explicit error.
- Self-catch: first draft had two near-duplicate verdict functions and
  passed uint64 to humanBytes(int64) — unified signature before commit.
- golangci-lint caught unchecked logRows.Columns() error; fixed by
  skipping rows whose columns can't be read.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=binlog_growth.
- Post-merge: verify npm v1.12.0 + docker tags published.
