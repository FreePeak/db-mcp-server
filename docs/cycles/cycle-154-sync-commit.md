# Cycle 154 — synchronous_commit Audit (performance action=synchronous_commit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL's counterpart to cycle 149's MySQL durability check:
  with `synchronous_commit=off` a COMMIT is acknowledged before WAL is
  flushed — an OS crash can lose recently committed transactions
  (~wal_writer_delay×3 of work). Frequently flipped off for
  write-heavy batch jobs and never turned back on; the loss only
  surfaces after the first crash. Confirmed absent from the tool
  surface.

## Shipped

- `internal/usecase/sync_commit.go`:
  - `syncCommitQuery` — current_setting('synchronous_commit');
    postgres/postgresql only.
  - `syncCommitVerdict` — pure classifier: off → WARNING naming the
    commit-loss window with ALTER SYSTEM fix (and per-session SET for
    batch jobs that accept the tradeoff); on → "" (audit adds the
    explicit clean line); local/remote_write → durable-with-standby-
    caveat note via `durabilityNote`; empty/unreadable → verify hint.
  - `AuditSyncCommit` — runs the probe against the live database;
    unsupported engines get an explicit error.
- Performance tool: new action `synchronous_commit` (both per-db and
  unified constructors) served via capability interface
  `syncCommitUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestSyncCommitProbe`: probe shape + engine gating.
  - `TestSyncCommitVerdict`: on renders empty; remote_apply durable;
    off escalated with "committed"/"lost" plus a named fix; empty
    value flagged unreadable.
  - `TestAuditSyncCommit_Unsupported`: explicit error for non-PG.
- Self-catch during RED→GREEN: "on" rendered a verdict where the
  design expects "" (clean line added by audit), and the off-warning
  lacked the literal "lost" substring — both aligned.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=synchronous_commit.
- Post-merge: verify npm v1.12.0 + docker tags published.
