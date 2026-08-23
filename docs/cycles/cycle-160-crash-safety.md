# Cycle 160 — Crash-Safety Audit (performance action=crash_safety)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- `fsync=off` and `full_page_writes=off` are commonly set for
  benchmarks and bulk loads, then forgotten in production:
  - fsync=off lets the OS discard acknowledged commits on power loss.
  - full_page_writes=off invites torn-page corruption after a crash.
  Silent data corruption is the worst failure mode to discover late.
  Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/crash_safety.go`:
  - `crashSafetyQuery` — current_setting('fsync') +
    current_setting('full_page_writes') in one round trip;
    postgres/postgresql only.
  - `truthySetting` — parses PostgreSQL GUC booleans ("on"/"t"/…).
  - `crashSafetyVerdict` — pure classifier with a three-way ladder
    (both off → corruption + data-loss WARNING; fsync off alone →
    commit-loss WARNING; full_page_writes off alone → torn-page
    WARNING; both on → "" so the audit adds the explicit clean line).
    Each branch names the ALTER SYSTEM fix.
  - `AuditCrashSafety` — runs the probe, renders verdict or healthy
    line; unsupported engines get an explicit error.
- Performance tool: new action `crash_safety` (both per-db and unified
  constructors) served via capability interface `crashSafetyUseCase`.

## Verification

- TDD RED first (build fail), then one wording fix in the test itself:
  the fsync-only case asserts "lost" (data loss) rather than
  "corruption" — only torn pages corrupt; semantics corrected before
  GREEN.
  - `TestCrashSafetyProbe`: probe shape + engine gating.
  - `TestCrashSafetyVerdict`: both-on empty; fsync-off escalated with
    ALTER SYSTEM fix; full_page_writes-off escalated with torn-page
    warning.
  - `TestAuditCrashSafety_Unsupported`: explicit error for non-PG.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=crash_safety.
- Post-merge: verify npm v1.12.0 + docker tags published.
