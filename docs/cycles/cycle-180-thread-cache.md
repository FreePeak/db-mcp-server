# Cycle 180 — thread_cache_size Churn Audit (performance action=thread_cache_size)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Every connection MySQL can't serve from the thread cache pays a
  thread-create/destroy cycle. Unlike config-only audits, this one is
  evidence-driven: `Threads_created` vs `Connections` from
  performance_schema.global_status is the observed miss rate, so the
  verdict reflects actual churn, not just the setting value.
- Escalation threshold: >5% of connections needed a new thread.
  Fix suggestion scales with current size (×4, clamped 16–100) and
  names the live SET GLOBAL + my.cnf persistence path plus the
  "watch Threads_created flatten" verification step.
- Zero connections renders an honest "no evidence yet" note instead
  of guessing.

## Shipped

- `internal/usecase/thread_cache.go`:
  - `threadCacheProbe` — joins @@GLOBAL.thread_cache_size with
    Connections/Threads_created counters; mysql/mariadb only.
  - `threadCacheVerdict` — pure classifier: >5% miss → WARNING with
    counts + fix; zero conns → no-evidence note; low churn → ""
    (audit adds explicit healthy line).
  - `AuditThreadCache` — runs the probe; unparseable counters render
    as unreadable (sentinel −1), never guessed at; unsupported
    engines get an explicit error.
- Performance tool: new action `thread_cache_size` (both per-db and
  unified constructors) served via capability interface
  `threadCacheUseCase`.

## Verification

- TDD RED first (build fail), GREEN on first run of implementation —
  no fixes needed this cycle.
- Tests: probe pairs setting with Threads_created + engine gating;
  low-churn quiet; 800/10000 escalated with counts and fix;
  pre-traffic honest note; explicit non-MySQL unsupported error.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=thread_cache_size.
- Post-merge: verify npm v1.12.0 + docker tags published.
