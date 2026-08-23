# Cycle 202 — MySQL binary-log retention audit

## Objective
Close the disk-fill gap from the storage audit: `table_sizes` reports data files, but nothing checks the **binary logs** — the classic silent killer where `binlog_expire_logs_seconds` defaults to 0 (never expire) and binlogs grow until the disk fills and the engine dies mid-write.

## Research findings
- MySQL 8.0 default for `binlog_expire_logs_seconds` is actually 2592000 (30 days), but many deployments (and MariaDB via `expire_logs_days`) still run with no expiry configured; replication setups that disable expiry "to be safe" are common.
- The failure mode is nasty: writes succeed until the disk is full, then every write fails with a cryptic error while reads keep working — looks like an app bug.
- `SHOW BINARY LOGS` gives per-file sizes plus `@@GLOBAL.binlog_expire_logs_seconds` gives retention in one round trip.

## What shipped (`internal/usecase/binlogs.go`, +tests)
- `AuditBinaryLogs(dbID)`: one catalog query returns file count, total bytes, and the configured expiry.
- `binlogVerdict`: three outcomes — no files (disabled or just rotated), **WARNING when expiry ≤ 0** ("set binlog_expire_logs_seconds before the disk fills"), healthy otherwise (files, humanized size, retention in days).
- Zero-expiry warns regardless of current size: small binlogs today are tomorrow's outage.
- Wired as `performance_<db_id>` action `audit_binlogs`; unsupported engines get the standard "not available" error.

## Verification
- `TestBinlogVerdict`: zero-files / zero-expiry warning / healthy-with-days matrix.
- `go test ./internal/usecase/ -run TestBinlog` green.

## Artifacts
- Commit on `hackathon`, pushed to `origin/hackathon`, PR #87.
