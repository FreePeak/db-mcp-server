# Cycle 134 — WAL Archiver Health (performance action=wal_archive)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- On archive_command deployments a failing archiver accumulates WAL
  segments locally until the disk fills and silently breaks point-in-time
  recovery — pg_stat_archiver's failure counters are the only warning,
  and nothing on the tool surface read them. Confirmed absent.

## Shipped

- `internal/usecase/wal_archive.go`:
  - `archiverQuery` — archived_count, failed_count, last_archived_wal,
    last_failed_wal from pg_stat_archiver; Postgres-only.
  - `archiverVerdict(archived, failed, lastArchivedWAL, lastFailedWAL)`
    — pure classifier: failures → FAILING with the failed segment name
    and the disk-full/PITR-broken consequence; healthy with counts;
    never-archived → archive_mode may be off hint.
  - `CheckWALArchive(ctx, dbID)`; other engines error "not available".
- Performance tool: new action `wal_archive` (both per-db and unified
  constructors) served via capability interface `walArchiveUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestArchiverCatalog`: hits pg_stat_archiver + failed_count +
    last_failed_wal; mysql/sqlite "".
  - `TestCheckWALArchive_Unsupported`: explicit error.
  - `TestArchiverVerdict`: healthy / FAILING-with-segment-name /
    never-archived escalation proven.
  - Test self-catch: my own test call had lastArchived/lastFailed WAL
    arguments swapped; the failing assertion exposed it before any
    implementation change was needed.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=wal_archive.
- Post-merge: verify npm v1.12.0 + docker tags published.
