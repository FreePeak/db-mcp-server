# Cycle 190 — track_counts Audit (performance action=track_counts)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `track_counts` defaults `on`, but when it's been turned off to
  shave CPU every `pg_stat_*` counter freezes at zero:
  - autovacuum goes blind — its thresholds come from `n_dead_tup` /
    `n_mod_since_analyze`, so bloat accumulates untriggered;
  - planner statistics go stale;
  - stats-based diagnostics (including this server's own
    idle_sessions / engine-stats actions) silently read zeros while
    looking healthy.
- The failure is invisible precisely because everything still
  "works" — which is why it needs its own audit rather than relying
  on downstream tools noticing zeros.
- Fix is reloadable: ALTER SYSTEM + pg_reload_conf().
- Candidate sweep: autovacuum_naptime / effective_io_concurrency /
  maintenance_work_mem / hot_standby_feedback / wal_buffers left as
  future candidates; default_statistics_target /
  checkpoint_completion_target / random_page_cost already covered.

## Shipped

- `internal/usecase/track_counts.go`:
  - `trackCountsProbe` — reads `current_setting('track_counts')`;
    postgres only.
  - `trackCountsVerdict` — pure classifier: on → "" (audit adds
    explicit healthy line); off → WARNING naming the frozen-counter
    blast radius + the reloadable fix; blank/other → unreadable
    note.
  - `AuditTrackCounts` — runs the probe; empty values fall to the
    unreadable note; unsupported engines get an explicit error.
- Performance tool: new action `track_counts` (both per-db and
  unified constructors) served via capability interface
  `trackCountsUseCase`.

## Verification

- TDD RED first (build fail), GREEN after implementation with no
  test edits needed this cycle.
- Tests: probe shape + engine gating; on quiet; off escalated with
  autovacuum blast radius + fix named; blank renders unreadable;
  explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=track_counts.
- Post-merge: verify npm v1.12.0 + docker tags published.
