# Cycle 148 — table_open_cache Pressure Audit (performance action=table_cache)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- MySQL keeps table definitions open per session; when the cache is
  saturated every access pays a close-and-reopen and Opened_tables
  climbs without bound. A saturated cache is one SET GLOBAL away from
  fixed — but only if someone looks. Confirmed absent.

## Shipped

- `internal/usecase/table_cache.go`:
  - `tableOpenCacheQuery` — @@GLOBAL.table_open_cache joined to the
    performance_schema Open_tables / Opened_tables counters;
    mysql/mariadb only.
  - `tableCacheVerdict` — pure classifier: saturated (full cache +
    churn > 2× cache) → WARNING naming the SET GLOBAL fix with a 2×
    suggestion; else healthy with slot usage.
- Performance tool: new action `table_cache` (both per-db and unified
  constructors) served via capability interface `tableCacheUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestTableCacheProbe`: probe shape + engine gating.
  - `TestTableCacheVerdict`: saturation escalation + healthy clean.
  - `TestAuditTableCache_Unsupported`: explicit error.
- Self-catch: golangci-lint errcheck flagged unchecked ParseInt — parse
  helper now logs and returns 0 (verdict treats as unreadable).
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=table_cache.
- Post-merge: verify npm v1.12.0 + docker tags published.
