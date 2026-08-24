# Cycle 47 — Oracle Init Scripts Land in the Right Database

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- Root-caused why `scripts/oracle-init/*.sql` never produced usable objects on fresh volumes: the gvenzl entrypoint executes user scripts connected **as SYSDBA in the CDB root**, so unqualified `CREATE TABLE` landed as `SYS.TEST_USERS` — invisible to both TESTDB and the `testuser` application schema. Cycle 44's live tests only worked because of manual fixes that this cycle deliberately wiped and replaced with working automation.
- Both init scripts now connect explicitly before creating anything:
  - `01-create-test-schema.sql` → `CONNECT testuser@//localhost:1521/TESTDB`, schema + seed data + sequence created as the app user.
  - `02-create-readonly-user.sql` → `CONNECT sys@…AS SYSDBA` in TESTDB (user/grant creation needs privileges; the PDB, not the CDB root, is where tests connect).

## Verification
- Full reproducibility test: `docker-compose down -v && up -d` from scratch — container boots healthy, both scripts execute without ORA-/SP2- errors, `test_users` holds 3 rows under `testuser` in TESTDB.
- Both Oracle read-only live tests pass against that fresh volume with zero manual steps (previously impossible).
- Full suite green uncached; smoke green over live stdio.

## Design Notes
- The failure mode was invisible in CI because the GitHub Actions workflow initializes the Oracle schema via an explicit sqlplus step — only local docker-compose development hit the silent misplacement. Fresh-volume verification is now part of the loop whenever these scripts change.
