# Cycle 168 — wal_compression Audit (performance action=wal_compression)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- With the default `wal_compression=off`, every checkpoint floods WAL
  with raw 8KB full-page images: replica shipping lags on volume,
  archive storage balloons, and backup windows all pay. PG14+ offers
  cheap `lz4`/`zstd` codecs; `pglz` works everywhere but costs more
  CPU. The setting is user-context, so the fix needs no restart —
  ALTER SYSTEM + pg_reload_conf() (or per-session/role SET).
- Adjacent coverage already shipped: checkpoint pressure
  (max_wal_size hit rate), crash_safety (fsync/full_page_writes),
  wal_level, timeout guardrails. This closes the WAL-volume gap.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/wal_compression.go`:
  - `walCompressionQuery` — current_setting('wal_compression');
    postgres only.
  - `walCompressionVerdict` — pure classifier: on/pglz/lz4/zstd → ""
    (audit adds the explicit clean line with the actual value); off →
    WARNING naming uncompressed full-page images and the ALTER SYSTEM
    + pg_reload_conf fix; empty/unreadable → verify note.
  - `AuditWalCompression` — runs the probe, renders verdict or
    healthy line; unsupported engines get an explicit error.
- Performance tool: new action `wal_compression` (both per-db and
  unified constructors) served via capability interface
  `walCompressionUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestWalCompressionProbe`: probe shape + engine gating.
  - `TestWalCompressionVerdict`: all four enabled codecs render
    empty; off escalated with full-page wording + ALTER SYSTEM/reload
    fix path; empty flagged unreadable.
  - `TestAuditWalCompression_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=wal_compression.
- Post-merge: verify npm v1.12.0 + docker tags published.
