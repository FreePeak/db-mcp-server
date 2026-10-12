// Package selfupdate implements the `db-mcp-server update` command: resolve the
// latest GitHub release, verify the platform binary against the release's
// SHA256SUMS manifest, and swap the running executable in place.
//
// The release layout it relies on is produced by .github/workflows/npm-publish.yml
// (assets db-mcp-server-<os>-<arch>[.exe] plus SHA256SUMS) and consumed by
// bin/install.js, so the checksum semantics here deliberately match
// install.js: findChecksum.
package selfupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository releases are published to.
const Repo = "FreePeak/db-mcp-server"

// maxDownloadBytes bounds a release asset so a wrong URL cannot exhaust memory.
const maxDownloadBytes = 256 << 20

// AssetName returns the release asset for a GOOS/GOARCH pair.
func AssetName(goos, goarch string) (string, error) {
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}

	var suffix string
	switch goos {
	case "darwin", "linux":
	case "windows":
		suffix = ".exe"
	default:
		return "", fmt.Errorf("unsupported platform %q", goos)
	}

	return fmt.Sprintf("db-mcp-server-%s-%s%s", goos, goarch, suffix), nil
}

// Compare returns -1, 0 or 1 for a against b, and false when either is not a
// parseable version. Numeric major.minor.patch decides first; a prerelease
// suffix (1.2.3-rc1) ranks below the same release.
func Compare(a, b string) (int, bool) {
	an, apre, aok := parse(a)
	bn, bpre, bok := parse(b)
	if !aok || !bok {
		return 0, false
	}
	for i := range an {
		switch {
		case an[i] < bn[i]:
			return -1, true
		case an[i] > bn[i]:
			return 1, true
		}
	}
	switch {
	case apre == bpre:
		return 0, true
	case apre == "":
		return 1, true
	case bpre == "":
		return -1, true
	default:
		return strings.Compare(apre, bpre), true
	}
}

// parse splits "v1.2.3-rc1+build" into [1 2 3] and "rc1".
func parse(v string) ([3]int, string, bool) {
	var nums [3]int
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	prerelease := ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		prerelease = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nums, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, "", false
		}
		nums[i] = n
	}
	return nums, prerelease, true
}

// ChecksumFor returns the lowercase SHA-256 for asset from a SHA256SUMS
// manifest. Entries may carry a `release/` path prefix (the workflow runs
// `sha256sum release/db-mcp-server-*`) and a binary-mode `*` marker.
func ChecksumFor(manifest []byte, asset string) (string, error) {
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == asset || filepath.Base(name) == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no SHA-256 checksum found for %s", asset)
}

// Client fetches release metadata and assets.
type Client struct {
	HTTP    *http.Client
	Repo    string
	BaseURL string // override for tests; empty means github.com
}

// NewClient returns a Client with sane timeouts.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 5 * time.Minute}, Repo: Repo}
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimSuffix(c.BaseURL, "/")
	}
	return "https://github.com"
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return Repo
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "db-mcp-server-self-update")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() //nolint:errcheck // the error response is discarded anyway
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return resp, nil
}

// LatestVersion returns the newest published release version, without the "v".
//
// The tag comes from the redirect GitHub issues for /releases/latest rather
// than the JSON API: the API allows only 60 unauthenticated requests per hour
// per IP, which a CLI behind a shared NAT exhausts, while the redirect costs
// one request and needs no credentials.
func (c *Client) LatestVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", c.base(), c.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "db-mcp-server-self-update")

	client := *c.http()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // the caller drains and closes the body

	location := resp.Header.Get("Location")
	if resp.StatusCode < 300 || resp.StatusCode > 399 || location == "" {
		return "", fmt.Errorf("GET %s: HTTP %d with no release redirect", url, resp.StatusCode)
	}
	tag := filepath.Base(strings.TrimSuffix(location, "/"))
	if tag == "releases" || tag == "latest" {
		return "", fmt.Errorf("%s has no published release", c.repo())
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// Asset downloads a release asset (a binary, or SHA256SUMS) for version.
func (c *Client) Asset(ctx context.Context, version, asset string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/releases/download/v%s/%s", c.base(), c.repo(), version, asset)
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // the body is drained by io.Copy above

	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", asset, err)
	}
	if n > maxDownloadBytes {
		return nil, fmt.Errorf("download %s exceeds %d bytes", asset, maxDownloadBytes)
	}
	return buf.Bytes(), nil
}

// Verify checks that binary matches the digest the release manifest claims.
func Verify(binary, manifest []byte, asset string) error {
	expected, err := ChecksumFor(manifest, asset)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(binary)
	if actual := hex.EncodeToString(sum[:]); actual != expected {
		return fmt.Errorf("SHA-256 mismatch for %s: manifest %s, download %s", asset, expected, actual)
	}
	return nil
}

// ExecutablePath returns the resolved path of the running binary, so a swap
// replaces the real file rather than a symlink pointing at it.
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

// Replace atomically swaps exePath for binary. The temp file is written beside
// the target so the rename stays on one filesystem; on any failure the original
// binary is left untouched.
//
// ponytail: Windows refuses to rename over a running executable. The caller
// surfaces the error and points at the npm installer, which is the supported
// path there. Upgrade: rename the running image aside and delete it on reboot
// (MoveFileEx MOVEFILE_DELAY_UNTIL_REBOOT) if Windows ever matters.
func Replace(exePath string, binary []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".db-mcp-server-update-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()        //nolint:errcheck // already closed on the success path
		_ = os.Remove(tmpName) //nolint:errcheck // no-op once the rename succeeded
	}()

	if _, err := tmp.Write(binary); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		return fmt.Errorf("replace %s: %w", exePath, err)
	}
	return nil
}

// Platform returns the running GOOS/GOARCH.
func Platform() (string, string) { return runtime.GOOS, runtime.GOARCH }
