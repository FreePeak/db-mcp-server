# Security Policy

## Supported versions

We provide security fixes for the latest released minor version only.
Always run the most recent release (see [releases](https://github.com/FreePeak/db-mcp-server/releases))
or build from `main` if you need an unreleased fix.

## Reporting a vulnerability

Please do **not** open a public issue for security problems.

This repository has GitHub **Private Vulnerability Reporting** enabled
(see issue #81). Report privately via:

**Security tab → Report a vulnerability**

<https://github.com/FreePeak/db-mcp-server/security/advisories/new>

Please include:

- A clear description of the issue and its impact
- The affected version(s) and configuration (e.g. database driver, deployment)
- Step-by-step instructions or a proof of concept to reproduce it
- A suggested fix, if you have one

## What to expect

Maintainers will acknowledge your report and respond within a reasonable
timeframe. We will work with you to understand the issue, coordinate a fix,
and credit you in the disclosure unless you prefer to remain anonymous.

Thank you for helping keep db-mcp-server and its users safe.

## Verifying releases

Release artifacts are signed keylessly with Sigstore, using the GitHub Actions
OIDC identity of the release workflow. No key or certificate file is needed.
Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
first, then replace `<version>` with the release number (for example `1.12.0`).

Container image (`docker-publish.yml`, signed by digest):

```bash
cosign verify \
  --certificate-identity https://github.com/FreePeak/db-mcp-server/.github/workflows/docker-publish.yml@refs/tags/v<version> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  docker.io/freepeak/db-mcp-server:<version>
```

Release binaries (`npm-publish.yml` signs `SHA256SUMS` and attaches the bundle
to the GitHub release):

```bash
mkdir -p release
gh release download v<version> --repo FreePeak/db-mcp-server --dir release

cosign verify-blob \
  --bundle release/SHA256SUMS.sigstore.json \
  --certificate-identity https://github.com/FreePeak/db-mcp-server/.github/workflows/npm-publish.yml@refs/tags/v<version> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  release/SHA256SUMS

# Run from the parent directory of release/, because the manifest lists release/<file>.
sha256sum --check release/SHA256SUMS
```
