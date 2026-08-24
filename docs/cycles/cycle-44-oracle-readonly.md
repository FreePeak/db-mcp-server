# Cycle 44 — Oracle Engine-Level Read-Only Enforcement (backlog #1 closeout)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Closes the last substantive backlog item:
- Root-caused why Oracle was blocked: `gvenzl/oracle-xe:21-slim` is amd64-only; under emulation on Apple Silicon it fails ORA-00442 at boot. Switched `docker-compose.oracle.yml` to `gvenzl/oracle-free:23-slim`, which ships native arm64+amd64 images — boots cleanly everywhere.
- Discovered empirically that Oracle has **no session-level read-only switch**: `ALTER SESSION SET READ ONLY` is ORA-02248. The engine enforces read-only through *privileges*, not flags.
- New fail-closed privilege audit (`pkg/db/oracle_readonly.go`): every pooled session on a `read_only` oracle database is audited before first use — `session_privs` for write-capable system privileges, `user_tab_privs_recd` for object-level INSERT/UPDATE/DELETE. Any write capability aborts Connect with remediation ("grant only CREATE SESSION plus SELECT grants"). Privileges are static per session, so verify-once means enforce-throughout; an unreadable catalog also refuses (fail closed).
- Live tests: restricted `mcp_ro` account (CREATE SESSION + SELECT only) serves reads and the engine itself rejects CREATE TABLE with ORA-01031; connecting as the schema owner with `read_only: true` must fail at Connect. Init script `02-create-readonly-user.sql` provisions the account on fresh volumes.
- Test-harness fixes along the way: Oracle has no `CREATE TABLE IF NOT EXISTS` and forbids leading-underscore identifiers.

## Design Notes
- First attempt used a hypothetical session flag; the live container rejected it within minutes of writing it — the value of validating against a real engine before shipping abstractions.

## Verification
- Both new live tests pass against a real Oracle Free 23 container; PG/MySQL read-only cases unchanged (skip when stack is down).
- Full suite green uncached (including TestOracleDataDictionary after aligning the local schema with the init script); smoke passes over live stdio; vet/gofmt clean.

## Backlog Impact
- Backlog #1 fully done — all four engines now enforce read_only at the engine layer.
