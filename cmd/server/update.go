package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/FreePeak/db-mcp-server/internal/selfupdate"
)

// version is the build version. Release builds set it with
// -ldflags="-X main.version=<package.json version>" (see
// .github/workflows/npm-publish.yml and the Makefile npm-release target); a
// plain `go build` leaves it as "dev".
var version = "dev"

// updateUsage is printed for `db-mcp-server update -h` and on bad flags.
const updateUsage = `Usage: db-mcp-server update [flags]

Replace this binary with the latest published release. The download is checked
against the release's SHA256SUMS manifest before the running executable is
swapped, and the previous binary is left untouched if verification fails.

Flags:
  -check            report whether a newer release exists, download nothing
  -force            reinstall even when the local version is current
  -version <v>      install a specific release instead of the latest
`

// runUpdate implements the `update` subcommand and returns a process exit code.
func runUpdate(args []string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, updateUsage) }
	check := fs.Bool("check", false, "report whether a newer release exists, download nothing")
	force := fs.Bool("force", false, "reinstall even when the local version is current")
	target := fs.String("version", "", "install a specific release instead of the latest")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ctx := context.Background()
	client := selfupdate.NewClient()

	latest := strings.TrimPrefix(*target, "v")
	if latest == "" {
		resolved, err := client.LatestVersion(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "db-mcp-server update: cannot resolve the latest release: %v\n", err)
			return 1
		}
		latest = resolved
	}

	fmt.Printf("current %s, latest %s\n", version, latest)

	current := strings.TrimPrefix(version, "v")
	if !*force {
		if cmp, ok := selfupdate.Compare(current, latest); ok && cmp >= 0 {
			fmt.Println("already up to date")
			return 0
		}
	}
	if *check {
		fmt.Println("a newer release is available; run 'db-mcp-server update' to install it")
		return 0
	}

	goos, goarch := selfupdate.Platform()
	asset, err := selfupdate.AssetName(goos, goarch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: %v\n", err)
		return 1
	}

	manifest, err := client.Asset(ctx, latest, "SHA256SUMS")
	if err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: cannot fetch SHA256SUMS: %v\n", err)
		return 1
	}
	binary, err := client.Asset(ctx, latest, asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: cannot fetch %s: %v\n", asset, err)
		return 1
	}
	if err := selfupdate.Verify(binary, manifest, asset); err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: refusing to install: %v\n", err)
		return 1
	}

	exePath, err := selfupdate.ExecutablePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: cannot locate the running binary: %v\n", err)
		return 1
	}
	if err := selfupdate.Replace(exePath, binary); err != nil {
		fmt.Fprintf(os.Stderr, "db-mcp-server update: %v\n", err)
		fmt.Fprintln(os.Stderr, "on Windows, update through the npm installer instead: npm install -g @freepeak/db-mcp-server")
		return 1
	}

	fmt.Printf("updated %s to v%s (SHA-256 verified); restart the server to use it\n", exePath, latest)
	return 0
}
