# Cycle 62 — Self-update command (`db-mcp-server update`) + a real `--version`

**Status:** ✅ Shipped · **Artifacts:** PR (this branch), `fix/self-update-command`

## Objective

The npm channel installs a release binary once. Nothing upgrades it: a user on
v1.12.0 has to know the release URL, know which of the seven platform assets
they need, download it, verify it by hand, and swap it into place — and the
binary could not even tell them which version they were running.

Close that: a version the binary can report, and an `update` command that
resolves the latest release, verifies it, and replaces the running executable.

## Findings

Two gaps, one root cause — the release version never reached the program:

- `cmd/server` declared **no `var version`**, so the
  `-ldflags="-X main.version=…"` the Makefile (`npm-release`) and
  `.github/workflows/npm-publish.yml` both pass was a **silent no-op**. Every
  release binary answered `1.0.0` — a literal in `cmd/server/main.go` and a
  second one in `internal/delivery/mcp/server_wrapper.go`.
- With no version, no update path is possible: the server cannot tell whether
  it is behind, and neither can the user.

The release layout to build against already existed and is consumed by
`bin/install.js`: assets `db-mcp-server-<os>-<arch>[.exe]` plus a `SHA256SUMS`
manifest whose entries carry a `release/` path prefix (cycle 61's bug). The
checksum semantics are re-implemented in Go, deliberately matching
`findChecksum` — a second parser that drifts is exactly how cycle 61 shipped
broken.

## What shipped

`internal/selfupdate` (new package) — the whole mechanism, stdlib only:

- `AssetName` maps GOOS/GOARCH to the release asset (rejects other platforms).
- `Compare` is a numeric semver compare (`1.9.0 < 1.10.0`), with prerelease
  ranking below its release, and reports "unparseable" for `dev`.
- `ChecksumFor` parses `SHA256SUMS` — `release/` prefix and binary-mode `*`
  marker included — and lowercases the digest.
- `Verify` is fail-closed: a mismatch aborts before anything is written.
- `Replace` writes a temp file **beside** the target (same filesystem), syncs,
  chmods 0755, then `rename`s — atomic, and a failure leaves the old binary
  intact.
- `LatestVersion` resolves the tag from the redirect GitHub issues for
  `/releases/latest`, not the JSON API: the API allows 60 unauthenticated
  requests per hour per IP, which a CLI behind a shared NAT exhausts.

`cmd/server/update.go` — the `update` subcommand, dispatched from `main`
before any flag parsing or config loading:

```
db-mcp-server update            # install the latest release over this binary
db-mcp-server update -check     # report whether a newer release exists
db-mcp-server update -force     # reinstall even when already current
db-mcp-server update -version X # install a specific release
```

`--version` prints the build version. `main.version` is now a real variable,
and the MCP `serverInfo.version` a client sees comes from it too
(`mcp.SetServerVersion`), so `1.0.0` is gone from both places.

## Verification (live, on this machine)

Built with the release ldflags (`-X main.version=1.13.0`) and driven directly:

```
$ ./db-mcp-server --version
1.13.0

$ ./db-mcp-server update -check
current 1.13.0, latest 1.13.0
already up to date

$ ./db-mcp-server update -force          # real network, real asset
current 1.13.0, latest 1.13.0
updated /private/tmp/dbmcp-live/db-mcp-server to v1.13.0 (SHA-256 verified); restart the server to use it
```

The downgrade path, which exercises resolve → download → verify → swap end to
end against the real v1.13.0 release:

```
$ ./db-mcp-server-v2 --version
1.12.0

$ ./db-mcp-server-v2 update
current 1.12.0, latest 1.13.0
updated /private/tmp/dbmcp-live/db-mcp-server-v2 to v1.13.0 (SHA-256 verified); restart the server to use it
```

The binary is replaced in place (the running process keeps the old inode; the
new file is the verified release asset). Stdio MCP session still handshakes
after the change, and now advertises the real version:

```
$ … initialize … | db-mcp-server -t stdio -c config.test.json
{"id":1,"jsonrpc":"2.0","result":{…"serverInfo":{"name":"DB MCP Server","version":"1.13.0"}}}
```

`internal/selfupdate/selfupdate_test.go` (11 cases, `go test -short ./...`
green, `go vet` clean, `gofmt` clean): asset naming per platform and rejection
of unsupported ones, semver ordering incl. the `1.9.0`/`1.10.0` trap and the
`v` prefix, checksum parsing against the real v1.13.0 manifest shape, a
tampered download being refused, `Replace` swapping content and preserving the
executable bit, a failed `Replace` (unwritable directory) leaving the original
untouched, and a fake release server driving resolve → download → verify.

## Notes and limits

- **Windows:** a running `.exe` cannot be renamed over, so `update` there fails
  with a pointer to the npm installer. Marked `ponytail:` in `Replace` with the
  upgrade path (`MoveFileEx … DELAY_UNTIL_REBOOT`) if Windows ever matters.
- The command replaces the binary **at its resolved path** (`EvalSymlinks`), so
  an npm `node_modules/.bin` symlink updates the real file rather than the link.
- No new dependency: `net/http`, `crypto/sha256`, `encoding/json` only.
- Unauthenticated GitHub calls are rate-limited by IP; the redirect endpoint
  costs one request and needs no credentials. A `GITHUB_TOKEN` is *not* used —
  the client sends no Authorization header, so no token is ever in play.
