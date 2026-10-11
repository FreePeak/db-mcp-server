# Cycle 60 — CI red on every push: secret guards no longer fail a branch build

**Status:** ✅ Shipped
**Commit:** (this branch)

## Objective

Every push to main produced two red runs — `docker` and `NPM Publish &
Release` — while `Go Build & Test` stayed green. The user reported it as
"CI pipeline failed"; the question was whether the code had regressed or the
CI was failing for a non-code reason.

## Findings

The failures are credential guards, not build failures:

| Workflow | Failing step | Cause |
|---|---|---|
| `docker` | `Check Docker Hub credentials` | `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` repository secrets are not set (0 secrets exist on the repo) |
| `NPM Publish & Release` | `Check npm credentials` | `NPM_TOKEN` repository secret is not set; `@freepeak/db-mcp-server` has never been published (registry 404) and the local token in `~/.npmrc` is 401 |
| `Go Build & Test` | — | green, including Build & Test, Integration Tests (MySQL/Postgres/Oracle/TimescaleDB containers) and Lint |

Both guards were added deliberately in cycle 11 (issue #86) so a publish
failure would appear as a `::error` with setup instructions instead of a late
`ENEEDAUTH`. The problem is scope: they run on **every** push, and a branch
push cannot publish anything anyway, so ~20 consecutive runs went red for a
config reason the code cannot fix. Docker Hub tags stop at 1.10/1.9 — nothing
has been published since 1.10 (2026-08-22), and the v1.12.0 tag never
produced an image.

Root cause: **a guard written for the publish event was attached to the
branch-push event.**

## Shipped

- `docker-publish.yml`: the credentials step now emits `ready=true/false` to
  `$GITHUB_OUTPUT`. Missing secrets on a tag push or a manual dispatch still
  `::error` + exit 1 (a real publish must not silently no-op); on a branch
  push it warns and the build runs with `push: false`, so the Dockerfile is
  verified without publishing. `Log in to Docker Hub` is gated on the same
  output, so no login is attempted with empty credentials.
- `npm-publish.yml`: same shape — `ready=true/false`; a branch push with no
  token warns and skips the publish step, a manual dispatch still fails hard.
- `issue #86` remains the action item for the user: add the three secrets.

## Verification

- `actionlint` on both workflows: no new findings vs the `origin/main`
  baseline (only the pre-existing SC2086/SC1135 style notes and the
  already-known old action versions remain).
- Both guard scripts executed directly under bash against the parsed
  workflow YAML, all 8 cases pass:
  - branch push, no secrets → exit 0, `ready=false`, `::warning::` (docker + npm)
  - tag push / manual dispatch, no secrets → exit 1, `::error::`
  - secrets present → exit 0, `ready=true`
  - only one of the two docker secrets present → treated as missing, exit 0, `ready=false`
- Push conditions re-checked: with `ready=false`, `login-action` is skipped and
  `build-push-action` runs with `push: false`; with `ready=true`, behaviour is
  unchanged from before.

## Fed Forward

Once `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN`/`NPM_TOKEN` land: re-run the
historical dispatches (v1.9.0/v1.10.0/v1.11.0 backfill, v1.12.0 tag) and
`npm view @freepeak/db-mcp-server version`; consider npm OIDC trusted
publishing so no long-lived token is needed (cycle 11's open item, still open).
