# Cycle 11 — Publish-Pipeline Integrity (Fail-Fast + Disclosure)

**Status:** ✅ Shipped · **Artifacts:** main `9db5aff`, issue #86

## Research Findings
- Following cycle 10's docker login failure, the npm channel was audited too: `ENEEDAUTH`, and `npm view freepeak-db-mcp-server` → **404**. The package listed in v1.9.0's changelog as a distribution channel has never existed on the registry.
- Both workflows burned full build minutes before dying with cryptic driver-level errors, which is why this stayed hidden since #48/#79.

## Shipped
- Preflight credential checks in both workflows emitting actionable `::error` messages (exact settings path, token creation URLs, re-run commands) instead of late-stage auth failures:
  - docker-publish.yml: validates `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN`
  - npm-publish.yml: validates `NPM_TOKEN`
- Issue #86 opened documenting both missing secret pairs and exact post-fix commands to publish v1.9.0/v1.10.0/v1.11.0 images and the 1.11.0 package.

## Verification
- YAML validated for both modified workflows; committed through pre-commit gates.

## Fed Forward
When secrets land (#86): verify Docker Hub tags + `npm view` shows 1.11.0; consider OIDC-based trusted publishing for npm to avoid long-lived tokens.
