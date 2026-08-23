# Cycle 153 — SQLite WAL-Mode Audit (performance action=wal_mode)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- SQLite's default rollback journal (delete mode) blocks readers while
  a write is in flight — under agent-driven concurrent access that
  surfaces as SQLITE_BUSY storms. WAL mode lets readers and one writer
  proceed concurrently and is one persistent pragma away
  (PRAGMA journal_mode=WAL). Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/wal_mode.go`:
  - `walModeQuery` — PRAGMA journal_mode; sqlite/sqlite3 only.
  - `walModeVerdict` — pure classifier: wal → healthy statement;
    memory/empty → in-memory note (WAL not applicable); other modes →
    escalation naming reader-blocking and SQLITE_BUSY, with the
    persistent PRAGMA fix.
  - `AuditWALMode` — runs the pragma against the live database and
    renders the verdict; unsupported engines get an explicit error.
- Performance tool: new action `wal_mode` (both per-db and unified
  constructors) served via capability interface `walModeUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestWALModeProbe`: probe shape + engine gating.
  - `TestWALModeVerdict`: wal healthy; delete escalated with
    "readers"/"busy"; memory noted as in-memory.
  - `TestAuditWALMode_EndToEnd`: audit runs against a real SQLite
    database via the standard test harness.
  - `TestAuditWALMode_Unsupported`: explicit error for non-SQLite.
- Self-catch during RED→GREEN: two verdict strings lacked lowercase
  "busy" / "in-memory" substrings the tests assert on (case-sensitive
  Contains) — messages updated to carry both forms.
- `gofmt` clean; `go build ./... && go vet ./... && go test ./...`:
  9/9 packages ok; `golangci-lint run` clean.

## Fed Forward

- README row for action=wal_mode.
- Post-merge: verify npm v1.12.0 + docker tags published.
