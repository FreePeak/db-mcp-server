# Cycle 136 — Checkpoint Pressure (performance action=checkpoint_pressure)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Checkpoints happen on schedule (timed) or because max_wal_size was
  hit (requested). When requested approaches timed, every write burst
  pays in latency spikes from aggressive checkpointing — the classic
  "writes stall periodically" tuning miss. PG17 moved the counters to
  pg_stat_checkpointer; older versions keep them in pg_stat_bgwriter.
  Confirmed absent.

## Shipped

- `internal/usecase/checkpoint.go`:
  - `checkpointQueries` — candidate SELECTs in preference order:
    pg_stat_checkpointer first, pg_stat_bgwriter legacy fallback second;
    empty for unsupported engines.
  - `checkpointVerdict(timed, requested)` — pure classifier: ≥20%
    requested → PRESSURE with the max_wal_size /
    checkpoint_completion_target hint; otherwise healthy with the split;
    zero counters → recently-started note.
  - `CheckCheckpointPressure(ctx, dbID)` — tries templates in order,
    first success wins; other engines error "not available".
- Performance tool: new action `checkpoint_pressure` (both per-db and
  unified constructors) served via capability interface
  `checkpointUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestCheckpointTemplates`: modern + legacy templates present in
    order; sqlite/mysql none.
  - `TestCheckpointVerdict`: healthy / PRESSURE-with-max_wal_size /
    empty escalation proven.
  - `TestCheckCheckpointPressure_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=checkpoint_pressure.
- Post-merge: verify npm v1.12.0 + docker tags published.
