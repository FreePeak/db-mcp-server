# Cycle 182 — max_wal_senders Capacity Audit (performance action=max_wal_senders)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `max_wal_senders` gates streaming replication and logical
  decoding. `0` disables both entirely; a server already using every
  sender slot rejects new standbys with "too many walsenders" during
  failover drills — the worst moment to discover it.
- Evidence-driven: live senders (pg_stat_replication) and reserved
  slots (pg_replication_slots) counted against the setting. Headroom
  is the minimum of `max - active` and `max - slots` (slots reserve
  sender capacity too); ≤0 means at capacity.
- Confirmed absent from the tool surface (wal_slots.go covers slot
  *retention*, not sender *capacity*).

## Shipped

- `internal/usecase/wal_senders.go`:
  - `walSendersProbe` — pairs current_setting with live usage
    counts; postgres only.
  - `walSendersVerdict` — pure classifier: negative → unreadable;
    0 → streaming-replication-disabled warning with ALTER SYSTEM +
    restart fix; at capacity → failover warning suggesting +2;
    headroom → "" (audit adds explicit healthy line).
  - `AuditWalSenders` — runs the probe; unparseable counters render
    as unreadable (sentinel −1), never guessed at; unsupported
    engines get an explicit error.
- Performance tool: new action `max_wal_senders` (both per-db and
  unified constructors) served via capability interface
  `walSendersUseCase`.

## Verification

- TDD RED first (build fail), then two GREEN fixes:
  1. Test-file typo (`t.Fatalf=` assignment instead of call) fixed.
  2. Real logic bug caught by the test: the slot-reservation branch
     overrode the active-sender deficit (`slots > free → free =
     max - slots`) letting 4 slots "free" a server with 5/5 active
     senders. Fixed to take the tighter constraint (minimum of both
     deficits).
- Tests: probe pairs setting with pg_stat_replication/pg_replication_slots
  counts + engine gating; headroom quiet; 0 escalated with fix;
  5 active / 4 slots on max=5 escalated as "at capacity"; negative
  unreadable; explicit non-PG unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=max_wal_senders.
- Post-merge: verify npm v1.12.0 + docker tags published.
