# Cycle 39 — Live-Engine Masking Validation (Masking Cycle C-lite)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Research Findings / Bug Caught
- The live test caught a real driver-shape bug the SQLite e2e could never expose: MySQL hands `VARCHAR` cells back as `[]byte`, and `maskPartial`'s `fmt.Sprintf("%v")` rendered those as decimal byte lists (`[53 53 49 …]`) before masking — output like 27 stars + ` 55]` instead of `******4567`. Fixed by decoding `[]byte` as text before any rune arithmetic, with a dedicated unit test.
- This is exactly the class of bug the throwaway-engine harness (cycle 33) exists for: engine-specific driver types are invisible to fakes.

## Shipped
- `TestExecuteQuery_MaskingLive`: seeds a PII scenario on both live engines via `scripts/live-db-setup.sh`, wraps the real connection with masking rules (fixed_string + partial), and asserts through each real driver: masked email present, phone keeping last 4 digits, no raw leakage, `Masked cells: 2` footer. Skips gracefully when engines are unreachable.
- Masking feature set now validated on all three tiers: unit → SQLite e2e → real PostgreSQL + MySQL.

## Verification
- Both subtests pass against real PostgreSQL and MySQL with zero skips; full suite green uncached; smoke passes; vet/gofmt clean; engines torn down cleanly.

## Fed Forward
- Masking is feature-complete per the scoping doc (Cycles A + B + this validation). Cycle C (table-qualified rules) stays optional pending a parser-dependency decision.
