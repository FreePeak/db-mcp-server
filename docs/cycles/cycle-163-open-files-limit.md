# Cycle 163 — open_files_limit Audit (performance action=open_files_limit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- MySQL's OS-level file-descriptor ceiling must supply the table
  cache, InnoDB files, and every connection. If `open_files_limit` is
  below roughly 2× `table_open_cache`, the cache is silently capped —
  evictions continue no matter how high the cache is tuned, and busy
  schemas hit "Too many open files". Unlike most settings this one
  cannot be raised with `SET GLOBAL`; it needs the config file (or
  ulimit) plus a restart. Cycle 152's table_cache audit watches
  eviction but nothing checked whether the ceiling can even supply the
  cache. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/open_files_limit.go`:
  - `openFilesLimitQuery` — @@GLOBAL.open_files_limit +
    @@GLOBAL.table_open_cache in one round trip;
    mysql/mariadb only.
  - `openFilesLimitVerdict` — pure classifier: ceiling < 2× cache →
    WARNING naming both values, the eviction/"Too many open files"
    consequence, and the my.cnf + restart fix (SET GLOBAL explicitly
    called out as not working for this one); zero/unreadable → verify
    note; comfortable → "" so the audit adds the explicit clean line.
  - `AuditOpenFilesLimit` — runs the probe, parses defensively,
    renders verdict or healthy line; unsupported engines get an
    explicit error.
- Performance tool: new action `open_files_limit` (both per-db and
  unified constructors) served via capability interface
  `openFilesLimitUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestOpenFilesLimitProbe`: probe shape + engine gating.
  - `TestOpenFilesLimitVerdict`: 100000/4000 renders empty; 1024/4000
    escalated naming the setting and requiring config+restart; zero
    flagged unreadable.
  - `TestAuditOpenFilesLimit_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=open_files_limit.
- Post-merge: verify npm v1.12.0 + docker tags published.
