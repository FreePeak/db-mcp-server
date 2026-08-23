# Cycle 150 — MySQL Strict-Mode Audit (performance action=strict_mode)

**Status:** Shipped · **Branch:** hackathon (PR #87)

## Research

- Without STRICT_TRANS_TABLES in sql_mode, MySQL silently truncates
  overlong strings, coerces invalid dates to the zero-date, and
  substitutes zeros for bad numerics — corruption that surfaces weeks
  later. Legacy servers and old docker images frequently ship without
  it. Confirmed absent from the tool surface.

## Shipped

- `internal/usecase/sql_mode.go`:
  - `sqlModeQuery` — @@GLOBAL.sql_mode; mysql/mariadb only.
  - `strictModeVerdict` — pure classifier: STRICT_TRANS_TABLES or
    STRICT_ALL_TABLES present → healthy statement; absent → WARNING
    naming silent truncation/coercion risk with the CONCAT-based SET
    GLOBAL fix and current mode echoed.
  - `AuditStrictMode` — renders the verdict; unsupported engines get
    an explicit error.
- Performance tool: new action `strict_mode` (both per-db and unified
  constructors) served via capability interface `strictModeUseCase`.

## Verification

- TDD RED first (build fail), then GREEN:
  - `TestStrictModeProbe`: probe shape + engine gating.
  - `TestStrictModeVerdict`: strict → healthy; empty/non-strict →
    WARNING with "silently"/"truncat" evidence.
  - `TestAuditStrictMode_Unsupported`: explicit error.
- `go build ./... && go vet ./... && go test ./...`: 9/9 packages ok;
  `golangci-lint run` clean.

## Fed Forward

- README row for action=strict_mode.
- Post-merge: verify npm v1.12.0 + docker tags published.
