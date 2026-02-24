# NPM Release Testing Guide

## Overview

This guide explains how to test the npm publish workflow and release automation for `@freepeak/db-mcp-server` before publishing to production.

## Architecture

The package uses a two-step installation approach:
1. **GitHub Releases** contain pre-built binaries for all platforms
2. **npm package** contains the install script that downloads the correct binary from GitHub

For **local testing**, the install script has fallback mechanisms:
- Uses pre-built binaries included in the test package
- Can build locally if Go is available
- Gracefully handles missing releases

## Testing Methods

### 1. Local Package Testing (Recommended)

Test the complete package without publishing:

```bash
# Build and create test package
make npm-test-local

# Install globally for testing
npm install -g /tmp/db-mcp-test/*.tgz

# Test the binary works
db-mcp-server --help

# Cleanup after testing
npm uninstall -g @freepeak/db-mcp-server
```

**What this tests:**
- ✅ Package contents and structure
- ✅ Install script execution
- ✅ Binary extraction and permissions
- ✅ Global installation
- ✅ Command availability

**What this doesn't test:**
- ❌ GitHub Release creation
- ❌ Actual npm registry publish
- ❌ Download from GitHub releases

### 2. Dry-Run Binaries Build

Test building all platform binaries:

```bash
# Build all release binaries
make npm-release

# Verify binaries were created
ls -lh release/

# Expected outputs:
# - db-mcp-server-darwin-amd64 (~18MB)
# - db-mcp-server-darwin-arm64 (~18MB)
# - db-mcp-server-linux-amd64 (~17MB)
# - db-mcp-server-linux-arm64 (~17MB)
# - db-mcp-server-windows-amd64.exe (~17MB)
```

### 3. Version Bump Testing

Test version management without committing:

```bash
# Test patch version bump (1.6.3 → 1.6.4)
make version-bump TYPE=patch

# Revert changes
git checkout package.json

# Or test other bump types
make version-bump TYPE=minor  # 1.6.3 → 1.7.0
make version-bump TYPE=major  # 1.6.3 → 2.0.0
```

### 4. Manual Install Script Testing

Test the install script directly:

```bash
# Build a binary first
go build -o bin/db-mcp-server ./cmd/server/main.go

# Run install script
node bin/install.js

# Verify it uses the existing binary
ls -lh bin/db-mcp-server
```

### 5. GitHub Actions Workflow Validation

The [.github/workflows/npm-publish.yml](.github/workflows/npm-publish.yml) workflow can be triggered manually:

```bash
# Via GitHub UI:
# 1. Go to Actions tab
# 2. Select "NPM Publish & Release" workflow
# 3. Click "Run workflow"
# 4. Select branch (main or test branch)

# Or via gh CLI:
gh workflow run npm-publish.yml
```

**Test branch workflow (without publishing):**

1. Create test branch:
   ```bash
   git checkout -b test-release-automation
   ```

2. Temporarily modify workflow to skip npm publish:
   ```yaml
   # In .github/workflows/npm-publish.yml
   # Comment out the npm publish step:
   # - name: Publish to NPM
   #   run: npm publish --access public
   ```

3. Push and test:
   ```bash
   git add .github/workflows/npm-publish.yml
   git commit -m "test: disable npm publish for testing"
   git push origin test-release-automation
   ```

4. Trigger manually via GitHub Actions UI

5. Verify:
   - ✅ Binaries build successfully
   - ✅ GitHub Release created
   - ✅ Release artifacts uploaded
   - ❌ npm publish skipped (as intended)

### 6. Full Release Workflow (Production)

**⚠️ Only run this when ready to publish!**

```bash
# Set NPM token (required)
export NPM_TOKEN=your_npm_token_here

# Run complete release workflow
make release TYPE=patch

# This will:
# 1. Bump version in package.json
# 2. Create git commit and tag
# 3. Build all platform binaries
# 4. Publish to npm
# 5. Push to GitHub (triggers GH Actions for release creation)
```

## Pre-Release Checklist

Before running the full release:

### Code Quality
- [ ] Run tests: `make test`
- [ ] Run linter: `make lint`
- [ ] Build succeeds: `make build`
- [ ] All errors fixed: Check VS Code problems panel

### Package Validation
- [ ] Run `make npm-test-local`
- [ ] Verify package size (<50MB): Check npm notice output
- [ ] Test global install: `npm install -g /tmp/db-mcp-test/*.tgz`
- [ ] Test binary execution: `db-mcp-server --help`
- [ ] Verify version in binary: `db-mcp-server --version` (if supported)

### Build Artifacts
- [ ] Run `make npm-release`
- [ ] Verify all 5 platform binaries created
- [ ] Check binary sizes are reasonable
- [ ] Test at least 2 platform binaries work

### Documentation
- [ ] Update [CHANGELOG.md](../CHANGELOG.md)
- [ ] Update [README.md](../README.md) if needed
- [ ] Verify version numbers are correct

### Credentials
- [ ] `NPM_TOKEN` environment variable set
- [ ] `NPM_TOKEN` secret configured in GitHub repo settings
- [ ] `GITHUB_TOKEN` has write permissions (automatic in GH Actions)

### Repository State
- [ ] All changes committed
- [ ] Working directory clean
- [ ] On `main` branch (or release branch)
- [ ] Pulled latest changes: `git pull origin main`

## Troubleshooting

### Install Script Fails with 404

**Problem:** `Failed to download: 404` when testing locally

**Solution:** This is expected for local testing. The install script will:
1. Try to download from GitHub releases (fails with 404 if not published)
2. Fall back to using pre-built binary from package (for local testing)
3. If both fail, provide manual installation options

For production installs, the GitHub release must exist first.

### Binary Not Found After Install

**Problem:** `db-mcp-server: command not found`

**Solutions:**
```bash
# Check if installed
npm list -g @freepeak/db-mcp-server

# Reinstall
npm uninstall -g @freepeak/db-mcp-server
npm install -g @freepeak/db-mcp-server

# Check npm bin path
npm bin -g
echo $PATH  # Verify npm bin is in PATH
```

### Package Size Too Large

**Problem:** npm package >50MB

**Solution:** Exclude unnecessary files via [.npmignore](../.npmignore) or `package.json files` field:
```json
{
  "files": [
    "bin/",
    "assets/",
    "config*.json",
    "README.md",
    "LICENSE"
  ]
}
```

### GitHub Actions Workflow Fails

**Problem:** Workflow fails at build or publish step

**Diagnostics:**
```bash
# Check workflow logs in GitHub Actions tab

# Test build locally
make npm-release

# Verify secrets are set
# Go to: Settings → Secrets and variables → Actions
# Verify: NPM_TOKEN is set
```

### Version Mismatch

**Problem:** package.json version doesn't match release

**Solution:**
```bash
# Check current version
node -p "require('./package.json').version"

# Manually set version
npm version 1.6.3 --no-git-tag-version

# Or use make target
make version-bump TYPE=patch
```

## Release Automation Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Developer runs: make release TYPE=patch                  │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Local tasks:                                              │
│    - Bump version in package.json                            │
│    - Git commit & tag                                        │
│    - Build binaries (optional, done in CI)                   │
│    - Publish to npm                                          │
│    - Git push to origin                                      │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. GitHub Actions Triggered (on push to main):              │
│    Job: get-version                                          │
│    - Extract version from package.json                       │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Job: build-and-release                                    │
│    - Setup Go 1.23                                           │
│    - Build binaries for all platforms                        │
│    - Create GitHub Release with tag v{version}              │
│    - Upload all platform binaries to release                │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Job: publish-npm (if enabled)                             │
│    - Setup Node.js                                           │
│    - Publish to npm registry                                 │
└───────────────────┬─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. User installs:                                            │
│    npm install -g @freepeak/db-mcp-server                    │
│                                                              │
│    - npm downloads package from npm registry                 │
│    - Runs bin/install.js (postinstall hook)                  │
│    - install.js downloads binary from GitHub release         │
│    - Binary placed in bin/db-mcp-server                      │
│    - Binary added to PATH via npm bin                        │
└─────────────────────────────────────────────────────────────┘
```

## Key Files

| File | Purpose |
|------|---------|
| [package.json](../package.json) | npm package configuration, version |
| [bin/install.js](../bin/install.js) | Downloads binary from GitHub releases |
| [bin/run.js](../bin/run.js) | Wrapper script that executes binary |
| [Makefile](../Makefile) | Build, test, and release automation |
| [.github/workflows/npm-publish.yml](../.github/workflows/npm-publish.yml) | CI/CD automation |

## Best Practices

1. **Always test locally first** with `make npm-test-local`
2. **Use semantic versioning**: patch for fixes, minor for features, major for breaking changes
3. **Update CHANGELOG.md** before each release
4. **Test on multiple platforms** if possible (macOS, Linux, Windows)
5. **Keep binaries under 50MB** if possible
6. **Tag releases** with descriptive release notes
7. **Monitor npm download stats** after publishing

## Emergency Rollback

If a bad version is published:

```bash
# Deprecate the version (doesn't remove it)
npm deprecate @freepeak/db-mcp-server@1.6.3 "Version broken, use 1.6.2"

# Users can install specific version
npm install -g @freepeak/db-mcp-server@1.6.2

# Quick fix and republish
make version-bump TYPE=patch
# Fix the issue
make release TYPE=patch
```

## References

- [npm documentation](https://docs.npmjs.com/)
- [GitHub Actions documentation](https://docs.github.com/actions)
- [Semantic Versioning](https://semver.org/)
- [GitHub Releases](https://docs.github.com/repositories/releasing-projects-on-github)
