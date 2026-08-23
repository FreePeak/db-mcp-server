# Cycle 198 — MySQL File-Access Surface Audit (performance action=file_access)
**Status:** Shipped · **Branch:** hackathon (PR #87)
## Research
- Two MySQL settings decide whether the server can be turned into a
  file reader/writer, and neither shows up in any other audit:
  - `local_infile=ON`: a compromised or malicious *client* can issue
    `LOAD DATA LOCAL INFILE` and have the **server** read any file the
    server process can access into a table — a classic
    privilege-escalation primitive.
  - `secure_file_priv=""` (empty): server-side `LOAD DATA INFILE` and
    `SELECT ... INTO OUTFILE` can touch arbitrary paths. `NULL`
    disables import/export entirely (safest); a dedicated path is the
    intended production posture.
- Both are one round trip to probe (`@@GLOBAL.local_infile`,
  `@@GLOBAL.secure_file_priv`) and both have unambiguous remediation,
  so they belong in the same single-action audit.
## Shipped

- `internal/usecase/file_access.go`:
  - `fileAccessQuery` — MySQL/MariaDB-only probe; other engines report
    unsupported like every engine-gated audit.
  - `parseInfileValue` — normalizes every driver shape for
    `local_infile` (`int64`, string, bytes, bool).
  - `fileAccessVerdict` — pure verdict: one WARNING line per risky
    setting with the exact `SET GLOBAL` / config fix; empty when clean.
  - `AuditFileAccess` — renders the warning lines, an explicit
    locked-down line naming the restricted path, or the NULL case
    called out as fully disabled.
- Registry: `{"file_access", AuditFileAccess}` appended to the cycle-197
  health-audit registry, so it also appears inside `health_audit`
  combined reports (skipped silently on non-MySQL engines).
- Performance tool: new action `file_access` (per-db + unified) via
  capability interface `fileAccessUseCase`; action-list descriptions
  updated.

## Verification

- TDD RED first (build failure), GREEN after implementation:
  - `TestFileAccessQuery` — mysql/mariadb supported, postgres/sqlite not.
  - `TestFileAccessVerdict` — full matrix: clean path, ON infile,
    empty secure_file_priv, NULL-disabled, and both-bad-reports-both;
    WARNING substrings asserted present/absent.
  - `TestParseInfileValue` — all ON/OFF shapes.
- `go build ./... && go vet ./internal/... && go test ./internal/usecase/ -count=1` ok.

---
*Cycle protocol: TDD red→green; registry + README updated per protocol.*
