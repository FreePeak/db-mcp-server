# Cycle 37 — Column Masking Hardening (Masking Cycle B)

**Status:** Shipped · **Artifacts:** main (this cycle)

## Shipped
Implements Cycle B of the masking scoping doc:
- **Fail-closed config validation:** `validateMaskingRules` in internal/config runs at startup — invalid regex patterns or unknown strategies abort config load with a message naming the database and rule index. Rationale: a broken pattern silently disables the mask it was meant to enforce; graceful degradation is exactly wrong when the operator believes data is hidden.
- **`partial` strategy:** keeps the trailing `keep_last` characters visible and stars the rest. Runes counted, not bytes (multibyte text never splits mid-character); values no longer than `keep_last` are fully masked so short cells cannot dodge the rule.
- **Observability:** `formatQueryResults` counts applied masks and appends `Masked cells: N (masking_rules active)` to the footer, so operators can confirm masking engaged from any result. Unknown strategies are reported as not-applied rather than counted.
- **README** guardrail row updated for all three strategies + validation semantics.

## Lessons During Verification
- Two test-expectation bugs were mine, not the code's: I miscounted rune lengths (`alice@example.com` → 13 stars, not 12) and mis-derived the multibyte case (`日本語です` keeps `本語です`, 4 runes). Writing exact expected strings forced me to actually verify the arithmetic instead of eyeballing it.
- A stale e2e assertion ("visible must remain readable") broke the moment a third masking rule legitimately covered that column — evidence the tests really exercise the rules rather than decorative checks.

## Verification
- Unit: partial exact-string cases including multibyte and shorter-than-keep_last; validation fail-closed cases (invalid pattern names db+rule index, unknown strategy rejected); matcher unchanged.
- E2E SQLite: three-rule query produces fixed_string mask, NULL mask, partial mask, readable id column, and `Masked cells: 3` footer.
- Full suite green uncached; smoke passes; vet/gofmt clean.

## Fed Forward (Cycle C — optional)
- Table-qualified rules; evaluate parser dependency then decide.
- Live-engine gated masking scenario via scripts/live-db-setup.sh if a real-engine proof is wanted.
