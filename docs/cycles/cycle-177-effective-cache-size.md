# Cycle 177 — effective_cache_size Audit (performance action=effective_cache_size)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The planner uses `effective_cache_size` to estimate how much of a
  table/index the OS page cache is likely to hold when costing index
  scans. The 4GB default under-assumes on modern RAM-heavy hosts,
  biasing plans toward seq scans the cache would have made cheap.
- Host-dependent advice like random_page_cost, so the warning names
  what to size against (total RAM minus other processes' working
  sets), states it reserves no memory, and gives the fix path:
  ALTER SYSTEM SET effective_cache_size='…' then pg_reload_conf().
- Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/effective_cache.go`:
  - `effectiveCacheProbe` — current_setting probe; postgres only.
  - `effectiveCacheVerdict` — pure classifier: ==4GB → WARNING with
    sizing guidance and fix path; tuned → "" (audit adds explicit
    clean line); empty → unreadable note.
  - `AuditEffectiveCache` — runs the probe, renders verdict or
    explicit healthy line; unsupported engines get an explicit
    error.
- Performance tool: new action `effective_cache_size` (both per-db
  and unified constructors) served via capability interface
  `effectiveCacheUseCase`.

## Verification

- TDD RED first (build fail), GREEN on first run of implementation —
  no fixes needed this cycle.
  - `TestEffectiveCacheProbe`: probe shape + engine gating.
  - `TestEffectiveCacheVerdict`: "24GB" quiet; "4GB" escalated with
    fix path; "" unreadable.
  - `TestAuditEffectiveCache_Unsupported`: explicit non-PG error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=effective_cache_size.
- Post-merge: verify npm v1.12.0 + docker tags published.
