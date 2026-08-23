# Cycle 31 — Configurable Risk Warning Threshold

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- Cycle 30's advisory threshold was hardcoded at high; operators tuning for sensitive environments may want warnings on every medium write, while high-volume automation may only want critical surfaced.

## Shipped
- `SetRiskWarnAt(level)` on the use case: low/medium/high/critical; invalid values fall back to default `high`. Post-execution advisories fire at or above the configured level. Thread-safe.

## Verification
- TDD RED-first: threshold matrix proving medium INSERT warns at `medium`, stays clean at default `high` and raised `critical`; invalid input falls back.
- Full suite (9 packages), vet, golangci-lint clean. Zero Docker.

## Fed Forward
- Wire to a server flag/config field when operator-facing surface is next touched.
