# Cycle 33 — Repeatable Live-Engine Setup Script

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
- `scripts/live-db-setup.sh {start|stop}`: turns the manual throwaway-engine procedure from cycles 31–32 into one command. Initializes and launches Homebrew PostgreSQL + MySQL on the compose-matching ports (15432/13306) with the credentials the live-gated tests expect, seeds the db1 orders scenario idempotently (including the forced-invalid index via catalog update and the recursive-CTE row seeding with raised depth limit), and tears everything down cleanly.
- Engine discovery checks PATH first, then common Homebrew locations.

## Lessons During Verification
- **Go test cache masks live-test state.** After engines came up, a re-run of `-run '_Live'` replayed the cached skip result from teardown time. Live tests must be invoked with `-count=1` — noted here; worth remembering whenever "tests pass but did they really run?"
- Two authoring bugs caught by actually running the script: a stray noop-guard line that would have executed `/etc`, and `seed_my` using `db1` before creating it (`Unknown database`). The script was only trustworthy after a full start → test → stop cycle.

## Verification
- Full lifecycle proven: start brings both engines up seeded; all `_Live` tests pass against real PostgreSQL 18.6 and MySQL 9.7.1 with zero skips; stop removes state and frees both ports.
- Full suite green uncached; smoke passes; vet/gofmt clean.

## Fed Forward
- Wire `live-db-setup.sh start && go test -count=1 -run _Live ./... && live-db-setup.sh stop` into CI once Docker-based runners exist; locally it already replaces the compose stack.
