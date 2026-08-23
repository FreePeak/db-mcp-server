# Cycle 172 — JIT Compiler Audit (performance action=jit)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- PostgreSQL's default `jit=on` hands queries crossing
  `jit_above_cost` (100000) to the LLVM compiler — and for short
  OLTP shapes the compilation overhead routinely exceeds the query
  itself, showing up as mysterious latency spikes on otherwise-cheap
  plans. Analytical warehouses with long-running queries are the case
  it was built for; most OLTP services should disable it.
- Sighup-context: ALTER SYSTEM SET jit='off' + pg_reload_conf(), no
  restart. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/jit.go`:
  - `jitQuery` — current_setting('jit'); postgres only.
  - `jitVerdict` — pure classifier: on → WARNING naming LLVM
    compilation overhead on OLTP shapes with the ALTER SYSTEM +
    pg_reload_conf fix (keep-on caveat for analytical workloads);
    off/other → "" (audit adds the explicit clean line);
    empty/unreadable → verify note.
  - `AuditJIT` — runs the probe, renders verdict or healthy line;
    unsupported engines get an explicit error.
- Performance tool: new action `jit` (both per-db and unified
  constructors) served via capability interface `jitUseCase`.

## Verification

- TDD RED first (build fail), then GREEN first try:
  - `TestJITProbe`: probe shape + engine gating.
  - `TestJITVerdict`: "off" renders empty; "on" escalated naming
    LLVM and the fix path; empty flagged unreadable.
  - `TestAuditJIT_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=jit.
- Post-merge: verify npm v1.12.0 + docker tags published.
