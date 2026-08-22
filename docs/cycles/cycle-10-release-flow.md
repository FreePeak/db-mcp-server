# Cycle 10 — Release Flow: Merge, Tag, Distribution

**Status:** ✅ Shipped (with one external blocker documented) · **Artifacts:** merged PR #85 → main `326424d`; tag v1.11.0; npm 1.11.0 publish run; issue #83 root-cause comment

## Research Findings
- PR #85 passed all quality gates (Build & Test, Integration Tests, Lint) — mergeable.
- Remote tag `v1.10.0` already existed as a historical release (issue #83 context); moving a published tag would rewrite history for consumers.
- npm distribution publishes on `package.json` changes to main (not on tags).

## Shipped
- Merged PR #85 into main (merge-commit style matching repo history).
- Tagged **v1.11.0** at the merge commit with full release notes; docker workflow auto-triggered.
- Backfilled the historical gap: dispatched docker builds for v1.9.0 and v1.10.0.
- Bumped npm package to 1.11.0 on main → npm-publish run triggered.
- Commented root cause on issue #83.

## Incident & Root Cause
- First version-bump commit shipped malformed JSON (`"1.11.0,` — perl edit ate the closing quote); caught by inspection, fixed in follow-up commit, JSON validated via node.
- **All docker runs fail at login**: `Username and password required`. The repo has no `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets — this is the true, never-diagnosed root cause of issue #59/#83's stale Docker Hub images. Secrets require maintainer-held Docker Hub credentials; exact remediation steps posted on #83.

## Verification
- Merge commit present on origin/main; v1.11.0 tag pushed; docker + npm runs queued (docker blocked solely by secrets).
- Local main == remote main after release commits.

## Fed Forward
Once secrets are set: re-run three backfill dispatches (commands in #83 comment). Consider adding a CI check that fails fast with a clear message when required secrets are absent.
