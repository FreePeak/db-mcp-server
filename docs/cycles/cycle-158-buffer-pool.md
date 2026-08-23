# Cycle 158 — innodb_buffer_pool_size Sizing Audit (performance action=buffer_pool)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- The health tool reports the InnoDB cache hit *ratio*, but a low
  ratio is only the symptom. Nothing compared the configured
  innodb_buffer_pool_size against actual data volume: when the pool
  holds a small fraction of data+indexes, every cold read hits disk.
  The classic fix is sizing to ~60% of host RAM (dynamically settable
  since MySQL 5.7). Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/buffer_pool.go`:
  - `bufferPoolQuery` — pool size + user-data bytes (excluding system
    schemas) in one UNION round trip; mysql/mariadb only.
  - `bufferPoolVerdict` — pure classifier over constant
    `bufferPoolWarnRatio` (25%): below → WARNING naming coverage %,
    readable sizes via existing `humanBytes`, and the SET GLOBAL fix;
    ≥25% or degenerate inputs render "" (audit adds the explicit clean
    line).
  - `AuditBufferPool` — runs the probe, parses both rows defensively,
    renders verdict or healthy-coverage line; unsupported engines get
    an explicit error.
- Performance tool: new action `buffer_pool` (both per-db and unified
  constructors) served via capability interface `bufferPoolUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestBufferPoolProbe`: probe shape (pool + information_schema in
    one round trip) + engine gating.
  - `TestBufferPoolVerdict`: 8GB/1GB renders empty; 128MB/20GB
    escalated with SET GLOBAL fix and readable sizes.
  - `TestAuditBufferPool_Unsupported`: explicit error for non-MySQL.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=buffer_pool.
- Post-merge: verify npm v1.12.0 + docker tags published.
