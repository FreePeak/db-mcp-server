# Cycle 173 — innodb_flush_neighbors Audit (performance action=flush_neighbors)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `innodb_flush_neighbors` default 1 groups flushing of adjacent
  dirty pages — a spinning-disk optimization amortizing seek cost.
  On SSD/NVMe (nearly all modern deployments) there is no seek
  penalty, so neighbor coalescing only batches writes the storage
  didn't ask for and adds latency spikes. Zero it on flash.
- Live fix: SET GLOBAL innodb_flush_neighbors=0; persist via my.cnf
  or SET PERSIST. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/flush_neighbors.go`:
  - `flushNeighborsQuery` — @@GLOBAL probe; mysql/mariadb only.
  - `flushNeighborsVerdict` — pure classifier: 0 → ""; ≥1 → WARNING
    naming the spinning-disk origin and the live SET GLOBAL +
    persistence path; <0 → unreadable note.
  - `parseSettingInt` — tolerant numeric parse for string-typed
    drivers; unrecognized shapes fall into the unreadable verdict.
  - `AuditFlushNeighbors` — scans via any-typed value with int64/
    string/[]byte conversion, renders verdict or explicit healthy
    line; unsupported engines get an error.
- Performance tool: new action `flush_neighbors` (both per-db and
  unified constructors) served via capability interface
  `flushNeighborsUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestFlushNeighborsProbe`: probe shape + engine gating.
  - `TestFlushNeighborsVerdict`: 0 renders empty; 1 escalated
    naming spinning-disk origin and the live fix.
  - `TestAuditFlushNeighbors_Unsupported`: explicit non-MySQL error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=flush_neighbors.
- Post-merge: verify npm v1.12.0 + docker tags published.
