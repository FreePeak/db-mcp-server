# Cycle 61 — npm install path was broken for every user (checksum prefix mismatch)

**Status:** ✅ Shipped · **Artifacts:** PR (this branch), `fix/npm-install-checksum`

## Objective

Issue #86 was narrowed to "npm needs a token". Before treating that as the last
blocker, verify the npm path actually works once a token lands — publish is not
the same as install. So: pack the tarball from `main`, install it, run it.

## Findings

Installing the packed tarball fails outright:

```
npm error command sh -c node bin/install.js
npm error Installing db-mcp-server v1.13.0 for darwin-arm64...
npm error Failed to install db-mcp-server: no SHA-256 checksum found for db-mcp-server-darwin-arm64
```

Root cause — a name mismatch between the two halves of the same release:

| Producer | Line | Output |
|---|---|---|
| `npm-publish.yml` → Build release binaries | `sha256sum release/db-mcp-server-* > release/SHA256SUMS` | `2ba669ff…  release/db-mcp-server-darwin-arm64` |
| `bin/install.js` → `findChecksum` | `if (name === asset)` | compares against bare `db-mcp-server-darwin-arm64` |

Every manifest entry carries a `release/` prefix, the installer compares the
whole field, so **no release could ever install** — the check throws before the
binary is even downloaded. Both the checksum generation and the checksum parser
arrived together in PR #88 (`bin/install.js` did not exist at v1.12.0), so this
has been broken since the code was written; it was never exercised because
nothing installed the tarball.

Second finding, same gap: `npm-publish.yml` only ran `npm pack --dry-run`, so
CI verified the tarball's *contents* and never its *behaviour*.

## Shipped

- `bin/install.js`: `findChecksum` now matches the bare file name as well as the
  full field (`name.split("/").pop() === asset`), keeping the existing
  binary-mode `*` marker handling. Also guards the top-level `install()` call
  behind `require.main === module` and exports the helpers, so the parser is
  testable without triggering a download.
- `bin/install.test.js`: 8 cases — prefixed entry, windows `.exe`, bare name,
  `*` marker, digest lowercasing, blank/comment lines, genuinely-absent asset,
  and wrong-architecture-of-same-platform (must still throw).
- `npm-publish.yml`: runs `node --test bin/install.test.js` in the `verify` job,
  and adds an **end-to-end install smoke check** to `publish-npm` — pack the
  tarball, `npm install` it, run `db-mcp-server --help`. That is the check that
  would have caught this class of bug.

## Verification

- RED first: 8/8 new tests fail against the unfixed `install.js` with the exact
  production error.
- GREEN after the fix: `node --test bin/install.test.js` → `# pass 8`, `# fail 0`.
- End to end on the real v1.13.0 release: `npm pack` → `npm install` succeeds,
  the postinstall downloads the darwin-arm64 binary and its SHA-256
  (`2ba669ff…`) matches the published `SHA256SUMS` entry, and the installed
  launcher runs (`db-mcp-server --help` prints usage, exit 0).
- `actionlint` clean on the modified workflow (no findings beyond the
  pre-existing style notes); `node --check` on both launchers still passes.
- The test file is not published: `package.json` `files` lists explicit paths, so
  `npm pack` still contains exactly 7 entries.

## Fed Forward

npm publishing is still blocked on `NPM_TOKEN` (#86) — unchanged by this cycle.
What changed is that the first real publish will now produce a package that
installs.
