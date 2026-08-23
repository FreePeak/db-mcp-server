# Cycle 28 — Per-Database Default Verbosity

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 27's verbosity was request-only; operators standardizing token spend across fleets need a config default — same governance shape as `mask_pii` (cycle 21).
- Precedence model: explicit client choice > per-database default > full. Defaults fill gaps; they never override explicit requests.

## Shipped
- `"verbosity": "normal"|"minimal"` per-database connection setting: JSON config → engine config → `Verbosity()` on pkg/db.Database and DatabaseAdapter passthrough.
- Usecase applies the default only when the client leaves verbosity unset (`full` sentinel); capability-detected via anonymous interface so test doubles stay untouched.
- README/CHANGELOG already document the parameter from cycle 26's contract guard pattern.

## Verification
- TDD RED-first at both layers: manager config mapping (4 engines), live SQLite through LoadConfig, use-case precedence matrix (no default → full; normal default + unset client → truncated; explicit minimal beats default).
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Auto-LIMIT injection deferred: regex-level SQL rewriting risks subquery LIMIT collisions; max_rows already bounds context. Needs a real parser to do safely.
- Release: 1.12.0 bump on PR #87 merge.
