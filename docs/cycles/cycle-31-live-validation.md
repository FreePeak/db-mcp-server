# Cycle 31 — Live-Engine Validation of Catalog SQL

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings
- Five cycles of PostgreSQL/MySQL catalog SQL (pg_stat_user_indexes, pg_index.indisvalid, stats_reset, pg_stat_activity FILTER, pg_settings casts) had only ever run against fakes and SQLite. Docker is unavailable in this environment, but Homebrew ships full PostgreSQL binaries — a throwaway `initdb` instance on port 15432 with compose-matching credentials (user1/password1/db1) makes the live-gated test pattern runnable locally.
- All seven catalog queries executed cleanly against PostgreSQL 18.6. No syntax fixes needed.

## Bug Found and Fixed
- Live data exposed PK noise: `orders_pkey` appeared in UNUSED findings with DROP advice. Primary-key indexes back a constraint and cannot be dropped independently, so the advice is wrong by construction.
  - PostgreSQL: unused-index query now joins `pg_index` and excludes `indisprimary`.
  - MySQL: formatter filters `PRIMARY` index names via `isPrimaryKeyIndex`.

## Shipped
- `TestDbHealth_Live`: seeds the duplicate/redundant/invalid scenario idempotently and asserts DUPLICATE + REDUNDANT + INVALID findings appear while `DROP INDEX orders_pkey` never does. Skips gracefully when unreachable, same as existing live tests.
- Promoted the `openLive` helper from a closure inside TestEngineSlowQueries_Live to package level so both live tests share it.

## Verification
- Live run against real PostgreSQL: all assertions pass; MySQL subtest skips cleanly (no local server).
- Full suite green across all packages; smoke passes; vet/gofmt clean. Throwaway instance stopped and removed.

## Fed Forward
- A local mysqld would let the MySQL digest/sys-schema queries get the same treatment; mysql-client is installed but no server binary was found.
- The seeding scenario could move into docker-compose.test.yml init scripts so CI gets it for free.
