package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "db-mcp-server-darwin-arm64"},
		{"darwin", "amd64", "db-mcp-server-darwin-amd64"},
		{"linux", "amd64", "db-mcp-server-linux-amd64"},
		{"windows", "amd64", "db-mcp-server-windows-amd64.exe"},
	}
	for _, c := range cases {
		got, err := AssetName(c.goos, c.goarch)
		if err != nil {
			t.Fatalf("AssetName(%q, %q): %v", c.goos, c.goarch, err)
		}
		if got != c.want {
			t.Errorf("AssetName(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}

	for _, bad := range [][2]string{{"plan9", "amd64"}, {"linux", "riscv64"}} {
		if _, err := AssetName(bad[0], bad[1]); err == nil {
			t.Errorf("AssetName(%q, %q) should fail", bad[0], bad[1])
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.12.0", "1.13.0", -1},
		{"1.13.0", "1.12.0", 1},
		{"1.13.0", "1.13.0", 0},
		{"v1.13.0", "1.13.0", 0}, // tags arrive with the v prefix
		{"1.13.0", "1.13.0-rc1", 1},
		{"1.13.0-rc1", "1.13.0", -1},
		{"1.9.0", "1.10.0", -1}, // numeric, not lexical
		{"1.13.1", "1.13.0", 1},
	}
	for _, c := range cases {
		got, ok := Compare(c.a, c.b)
		if !ok {
			t.Fatalf("Compare(%q, %q) failed to parse", c.a, c.b)
		}
		if got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}

	if _, ok := Compare("dev", "1.13.0"); ok {
		t.Error(`Compare("dev", ...) must report an unparseable version`)
	}
}

// The release workflow runs `sha256sum release/db-mcp-server-*`, so every
// manifest entry carries a release/ prefix while the downloaded asset is the
// bare name. This is the same shape bin/install.test.js pins for the npm
// installer — the two parsers must not drift apart.
const releaseSums = `302c610d8299d6c7161fb8c01d3e00bb2855efd1c2a8a4ce61c16880db84fef5  release/db-mcp-server-darwin-amd64
2ba669ff628649bef9bbb8e9704c3e9ee72202ba078c1219300758df52edc45a  release/db-mcp-server-darwin-arm64
71310222bb17f715d5edd7edb0fe4be71a3f8573028c0cd2b0087dc894a1fd9d  release/db-mcp-server-linux-amd64
a991c8cf40a64c83c2f97f138c5a40b1277e9e512ffb7efd0313bdddee2a223c  *release/db-mcp-server-windows-amd64.exe
`

func TestChecksumFor(t *testing.T) {
	got, err := ChecksumFor([]byte(releaseSums), "db-mcp-server-darwin-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if want := "2ba669ff628649bef9bbb8e9704c3e9ee72202ba078c1219300758df52edc45a"; got != want {
		t.Errorf("ChecksumFor = %q, want %q", got, want)
	}

	// Binary-mode star marker, and a bare name with no prefix.
	got, err = ChecksumFor([]byte(releaseSums), "db-mcp-server-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a991c8cf40a64c83c2f97f138c5a40b1277e9e512ffb7efd0313bdddee2a223c"; got != want {
		t.Errorf("ChecksumFor with star marker = %q, want %q", got, want)
	}

	bare := "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789  db-mcp-server-linux-arm64\n"
	got, err = ChecksumFor([]byte(bare), "db-mcp-server-linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if want := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"; got != want {
		t.Errorf("ChecksumFor must lowercase the digest: got %q, want %q", got, want)
	}

	if _, err := ChecksumFor([]byte(releaseSums), "db-mcp-server-linux-arm64"); err == nil {
		t.Error("ChecksumFor must fail for an asset the manifest does not list")
	}
}

func TestVerify(t *testing.T) {
	binary := []byte("pretend this is a 30 MB go binary")
	sum := sha256.Sum256(binary)
	manifest := []byte(hex.EncodeToString(sum[:]) + "  release/db-mcp-server-linux-amd64\n")

	if err := Verify(binary, manifest, "db-mcp-server-linux-amd64"); err != nil {
		t.Fatalf("Verify rejected a matching download: %v", err)
	}
	if err := Verify([]byte("tampered"), manifest, "db-mcp-server-linux-amd64"); err == nil {
		t.Error("Verify must reject a download whose digest does not match")
	}
}

func TestReplaceSwapsBinaryAndKeepsMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "db-mcp-server")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(target, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("Replace left %q in place, want %q", got, "new")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("Replace dropped the executable bit: mode %v", info.Mode())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".db-mcp-server-update-") {
			t.Errorf("Replace left its temp file behind: %s", e.Name())
		}
	}
}

func TestReplaceLeavesTargetIntactWhenDirectoryIsUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "db-mcp-server")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := Replace(target, []byte("new")); err == nil {
		t.Fatal("Replace should fail when it cannot create a temp file")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Errorf("a failed Replace must not touch the existing binary; got %q", got)
	}
}

// fakeRelease serves the GitHub endpoints Client talks to, so the whole
// resolve -> download -> verify path is exercised without network access.
func fakeRelease(t *testing.T, version string, binary []byte) *httptest.Server {
	t.Helper()
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no release asset for this platform: %v", err)
	}
	sum := sha256.Sum256(binary)
	manifest := hex.EncodeToString(sum[:]) + "  release/" + asset + "\n"

	mux := http.NewServeMux()
	// GitHub answers /releases/latest with a 302 to /releases/tag/vX.Y.Z.
	mux.HandleFunc("/FreePeak/db-mcp-server/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/FreePeak/db-mcp-server/releases/tag/v"+version, http.StatusFound)
	})
	mux.HandleFunc("/FreePeak/db-mcp-server/releases/download/v"+version+"/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(manifest))
	})
	mux.HandleFunc("/FreePeak/db-mcp-server/releases/download/v"+version+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binary)
	})
	return httptest.NewServer(mux)
}

func TestClientResolvesAndDownloadsVerifiedAsset(t *testing.T) {
	binary := []byte("freshly downloaded binary")
	server := fakeRelease(t, "1.13.0", binary)
	defer server.Close()

	client := &Client{HTTP: server.Client(), Repo: Repo, BaseURL: server.URL}
	ctx := context.Background()

	latest, err := client.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if latest != "1.13.0" {
		t.Errorf("LatestVersion = %q, want %q (the v prefix must be stripped)", latest, "1.13.0")
	}

	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	manifest, err := client.Asset(ctx, latest, "SHA256SUMS")
	if err != nil {
		t.Fatalf("Asset(SHA256SUMS): %v", err)
	}
	got, err := client.Asset(ctx, latest, asset)
	if err != nil {
		t.Fatalf("Asset(%s): %v", asset, err)
	}
	if err := Verify(got, manifest, asset); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if string(got) != string(binary) {
		t.Error("downloaded bytes do not match what the server served")
	}
}

func TestClientFailsOnMissingRelease(t *testing.T) {
	server := fakeRelease(t, "1.13.0", []byte("x"))
	defer server.Close()

	client := &Client{HTTP: server.Client(), Repo: Repo, BaseURL: server.URL}
	if _, err := client.Asset(context.Background(), "9.9.9", "SHA256SUMS"); err == nil {
		t.Error("Asset must fail for a version that does not exist")
	}
}
