# Cycle 03 — Engine-Level Read-Only Enforcement

**Status:** ✅ Shipped · **Artifacts:** PR #85, commit `0ccb4d5`

## Research Findings
- Cycle 01's classifier is application-layer only; a bug there exposes writes. DBHub shipped dual-layer enforcement (keyword classifier + engine) in 0.22.6 for exactly this reason.
- Signature-preserving approach chosen: enforce in the DSN/session so pooled connections inherit it, avoiding invasive Query-signature refactors.

## Shipped
- PostgreSQL/TimescaleDB: DSN gains `options='-c default_transaction_read_only=on'` when `read_only: true`; skipped if operator supplies their own options string. (Learned live: lib/pq requires single-quoting values containing spaces.)
- MySQL: `transaction_read_only=1` DSN param — go-sql-driver applies unknown params as system variables per pooled connection. MySQL DSN construction extracted into `buildMySQLConnStr`.
- SQLite already enforced via `mode=ro`; Oracle documented as classifier + least-privilege for now.

## Verification
- Live against docker-compose.test.yml (MySQL 8 @13306, Postgres 15 @15432): server rejects `CREATE TABLE` on read-only connections for both engines while `SELECT 1` works (`TestReadOnlyEngineEnforcement_Live`, auto-skips without containers).
- DSN unit locks (`readonly_dsn_test.go`) hold behavior without containers.

## Fed Forward
Oracle enforcement deferred (needs live Oracle container); logged in backlog.
