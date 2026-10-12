# Cycle 60 — Release v1.13.0

**Status:** Shipped · **Artifacts:** tag `v1.13.0`, PR cutting the release

## Shipped
- Cut **v1.13.0**: 15 commits since v1.12.0. CHANGELOG `[Unreleased]`
  promoted and completed with everything that landed after it was last
  touched:
  - **PR #88** hardening: single canonical config object (fixes
    max_rows/masking/query_timeout silently dropped by the second config
    parser), strict JSON validation, real transaction lifetimes, SDK-backed
    streamable HTTP with Origin allowlisting + API-key auth + OIDC,
    unprivileged container, removal of the arbitrary server-directory `list`
    tool, and `Verify source` CI gating the publish workflows.
  - **PR #92** (superseding #89): `GetToolType` exact-name lookup before the
    underscore-prefix split, with a regression test pinning the collision.
  - TimescaleDB cycles 55–59: `list_hypertables` + discovery tools,
    lazy-loading-safe registration, live-engine catalog rewrites.
- package.json bumped 1.12.0 → 1.13.0 (npm release workflow validates the
  tag against it).

## Release-gate history
- PR #88's guard rescope (credential checks only on tag/dispatch events)
  superseded PR #91 on arrival: main pushes are green without secrets, and
  `npm-publish.yml` no longer triggers on branch pushes. #91 closed with the
  reasoning recorded on issue #86.

## Distribution status (still blocked — issue #86)
- The repository has **zero Actions secrets**, so the `v1.13.0` tag's
  Docker and npm publish jobs fail fast with an actionable `::error` — by
  design; a real publish must not silently no-op.
- Unblock: add `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `NPM_TOKEN` under
  Settings → Secrets and variables → Actions, then re-run the tag workflow
  and the backfills recorded in #86 (`gh workflow run docker-publish.yml
  -f tag=v1.9.0`, likewise v1.10.0/v1.11.0).
