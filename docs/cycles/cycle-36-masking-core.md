# Cycle 36 — Column Masking Core (Masking Cycle A)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Implements Cycle A of the column-masking scoping doc (cycle 35):
- **Config:** per-database `masking_rules` (`pattern` regex on output column names, `strategy`: `fixed_string` with `value` / `null`), plumbed `DatabaseConnectionConfig` → internal `Config` → `database.MaskingRules()` → repository adapter → use case, following the exact `max_rows`/`query_timeout` guardrail pattern.
- **Enforcement point:** one pass inside `formatQueryResults` — columns resolved to rules once per query (precompiled regexes), cells masked before rendering, so sensitive values never leave the process and every query shape including `SELECT *` is covered.
- **Matcher semantics:** case-insensitive by convention of the pattern author, first matching rule wins, invalid patterns never match (config validation hardening is Cycle B), unknown strategies pass through so a typo degrades to visible data rather than silent corruption.
- **README:** guardrail-table row + config example.

## Design Notes
- Two name collisions bit during wiring: a helper parameter named `db` shadowed the new `pkg/db` package import in the use case, and the repository lacked the import — both caught by build, not review.
- `formatQueryResults` signature grew a `masks []db.MaskingRule` parameter; existing maxrows tests updated with explicit `nil`.

## Verification
- Unit: matcher (case-insensitive match, first-wins, no-match → nil, invalid pattern degrades), strategies (fixed_string replaces even nil cells, null blanks, unknown passes through).
- E2E SQLite through `ExecuteQuery` with configured rules: masked email, tax_id as NULL, unmasked column readable; rule-less databases byte-identical to before.
- Full suite green uncached; smoke passes; vet/gofmt clean.

## Fed Forward (Cycle B)
- Config validation at load (invalid regex should fail fast, not silently never match).
- `partial` strategy with type coercion (keep last-4 etc.).
- Masked-cell count surfaced in response metadata for observability.
- Live-engine gated test via scripts/live-db-setup.sh once a masked scenario is seeded there.
