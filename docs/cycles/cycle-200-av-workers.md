# Cycle 200 — autovacuum_max_workers audit

**Status:** ✅ Shipped (hackathon branch)

## Objective

Close the last dimension of the PostgreSQL autovacuum audit trio (naptime
→ throttle → workers): `autovacuum_max_workers` is 3 by default and is
shared across every database in the cluster. Busy clusters queue vacuum
passes behind each other while dead tuples accumulate — exactly what the
cycle-199 bloat audit then reports. The audit should catch undersized
worker pools before bloat appears.

## Research findings

- PG default is 3 workers; each worker holds a connection slot and each
  database waits its turn, so many-database or write-heavy clusters
  starve vacuum with stock settings.
- Fix path is `ALTER SYSTEM SET autovacuum_max_workers = N` + restart;
  `max_worker_processes` must be raised to match if it was lowered.
- MySQL/SQLite have no equivalent setting → engine-gated probe.

## What shipped

- `internal/usecase/av_workers.go`: `AuditAVWorkers` — probes
  `current_setting('autovacuum_max_workers')`, escalates ≤3 workers with
  exact remediation SQL; ≥4 stays quiet (explicit clean line). Zero/
  unparseable renders "unreadable", never a false all-clear.
- Registered as health_audit action `av_workers` (registry-only, same
  as naptime/throttle/bloat).

## Verification evidence

- `TestAVWorkersProbe`: postgres-only probe.
- `TestAVWorkersVerdict`: escalation ladder {1,2,3→WARNING; 4,8→silent};
  verdict names the setting, ALTER SYSTEM fix, and max_worker_processes
  caveat; 0 renders "unreadable".
- RED confirmed first (build fail), GREEN after threshold correction —
  the test caught an initial off-by-one quiet floor (5 vs 4) and forced
  the implementation to match the spec.
- Full `go build ./... && go vet && go test ./...` green.

## Artifacts

- Commit on `hackathon` branch, pushed to origin.
