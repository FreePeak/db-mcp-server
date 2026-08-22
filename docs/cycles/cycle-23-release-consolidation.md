# Cycle 23 — Release Consolidation: CHANGELOG + PR to main

**Status:** ✅ Shipped · **Branch:** hackathon → PR opened against main

## Research Findings
- Six cycles (17–22) of shipped features existed only on the hackathon branch: invisible to release automation, unreviewable, and absent from the changelog that npm/docker consumers read.
- Release-flow research from cycles 10–11 established this repo's pattern: merge via PR, tag, CI publishes. The loop's output was blocking itself by not entering that pipeline.

## Shipped
- CHANGELOG `[Unreleased]` gains entries for: PII masking (opt-in + operator-enforced + Luhn precision) and `suggest_indexes` (alias-safe, composite, PK-aware).
- Pull request `hackathon → main` opened with a structured summary of all six cycles; branch pushed continuously after every cycle so CI runs per-cycle.

## Verification
- `gh pr create` succeeded; PR carries the cycle index. Full suite green at push time (pre-commit hooks re-verify gofmt + golangci-lint on every commit).

## Fed Forward
- On PR merge (or manual tag): bump package.json past 1.11.0 for the masking/advisor features.
- Masking audit log remains open.
