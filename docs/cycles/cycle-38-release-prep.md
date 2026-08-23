# Cycle 38 — Release Prep: v1.12.0 Cut

**Status:** ✅ Shipped · **Branch:** hackathon (PR #87)

## Research Findings
- npm publish triggers on push to main when package.json/go files change; it reads the version from package.json and skips if the tag already exists — so bumping on this branch makes the merge itself the release event.
- The repo's CHANGELOG had `[Unreleased]` misplaced mid-file (between v1.7.0 and v1.6.x) from an earlier backfill.

## Shipped
- `package.json` 1.11.0 → **1.12.0**: merge will auto-publish npm + trigger docker pipeline.
- CHANGELOG: `[Unreleased]` content (cycles 01–37 features: guardrails, explain/describe/health tools, ERD, index advisor, PII masking, verbosity, snapshots, drift detection, dry-run risk analysis) cut as dated **[v1.12.0] - 2026-08-22** at the top; fresh empty `[Unreleased]` beneath; version ordering restored newest-first.
- No Go code changes; no version constants in Go sources to bump.

## Verification
- CHANGELOG structure asserted by inspection (section order + content intact); build unaffected.
- Full suite green; zero Docker.

## Fed Forward
- After merge: verify npm shows 1.12.0 and docker tags appear; then tag v1.12.0 if automation requires manual tag push.
- Next feature cycles resume on new baseline.
