# Cycle 162 — shared_buffers Sizing Audit (performance action=shared_buffers)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL's `shared_buffers` default is a famously undersized
  128MB. When the pool holds only a sliver of the database, every cold
  read hits disk and hit ratios suffer no matter how much RAM the host
  has. The classic fix is ~25% of host RAM and it requires a restart,
  not just a reload. This is the PG counterpart of cycle 158's MySQL
  buffer_pool audit. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/shared_buffers.go`:
  - `sharedBuffersQuery` — pool bytes + `pg_database_size(current_)`
    in one UNION round trip; postgres/postgresql only.
  - `parsePrettySize` — parses pg_size_pretty output ("128 MB",
    "20 GB", "512 kB", plain bytes) back to bytes.
  - `sharedBuffersVerdict` — pure classifier over constant
    `sharedBuffersWarnRatio` (25%): below → WARNING naming coverage %,
    readable sizes via existing `humanBytes`, the ALTER SYSTEM sizing
    guidance, and the explicit note that this one needs a restart;
    ≥25% or degenerate inputs render "" (audit adds the explicit clean
    line).
  - `AuditSharedBuffers` — runs the probe, parses both rows
    defensively, renders verdict or healthy-coverage line;
    unsupported engines get an explicit error.
- Performance tool: new action `shared_buffers` (both per-db and
  unified constructors) served via capability interface
  `sharedBuffersUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestSharedBuffersProbe`: probe shape (pool + database size in one
    round trip) + engine gating.
  - `TestSharedBuffersVerdict`: 8GB pool/1GB data renders empty;
    128MB/20GB escalated with ALTER SYSTEM fix and readable sizes.
  - `TestAuditSharedBuffers_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=shared_buffers.
- Post-merge: verify npm v1.12.0 + docker tags published.
