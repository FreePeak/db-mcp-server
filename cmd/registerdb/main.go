// registerdb validates a cloud database connection string and registers it
// for the test suite. This is the human-facing half of cloud-DB
// auto-registration: sign up at any free provider (Neon, Supabase, Aiven,
// TiDB Cloud — all no credit card), copy the connection string, run:
//
//	go run ./cmd/registerdb my_neon "postgresql://...?sslmode=require"
//
// The DSN is live-pinged through the same pkg/db layer the server uses;
// only reachable databases are saved to .test-cloud-db.json (gitignored).
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "-list" || os.Args[1] == "--list") {
		list()
		return
	}
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, `Usage:
  registerdb <name> <dsn>   validate + register a cloud database
  registerdb -list          show registered cloud databases

Free providers (no credit card): Neon (neon.tech), Supabase,
Aiven (aiven.io), TiDB Cloud Serverless.
Env shortcut: export NEON_DATABASE_URL=... — tests pick it up automatically.
`)
		os.Exit(2)
	}
	register(os.Args[1], os.Args[2])
}

func register(name, dsn string) {
	cfg, err := db.ParseDSN(dsn)
	if err != nil {
		fatal(err)
	}

	fmt.Printf("Validating %s://%s:***@%s:%d/%s ... ",
		cfg.Type, cfg.User, cfg.Host, cfg.Port, cfg.Name)

	database, err := db.NewDatabase(cfg)
	if err != nil {
		fatal(fmt.Errorf("build connection: %w", err))
	}
	if err := database.Connect(); err != nil {
		fatal(fmt.Errorf("connect failed (free tiers may be asleep — wake it and retry): %w", err))
	}
	defer func() {
		if cerr := database.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: close failed: %v\n", cerr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := database.Ping(ctx); err != nil {
		fatal(fmt.Errorf("ping failed: %w", err))
	}
	fmt.Println("OK")

	if err := db.RegisterCloudDB(db.DefaultCloudRegistryPath, name, dsn); err != nil {
		fatal(err)
	}
	fmt.Printf("Registered %q (%s) in %s\n", name, db.DetectProviderName(cfg.Host), db.DefaultCloudRegistryPath)
}

func list() {
	reg, err := db.LoadCloudRegistry(db.DefaultCloudRegistryPath)
	if err != nil {
		fatal(err)
	}
	env := db.ConfigsFromEnv()
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	mustPrint := func(format string, args ...interface{}) {
		if _, err := fmt.Fprintf(w, format, args...); err != nil {
			fatal(err)
		}
	}
	mustPrint("NAME\tPROVIDER\tTYPE\tHOST\tSOURCE\n")
	for _, e := range env {
		mustPrint("%s\t%s\t%s\t%s\tenv\n", e.Name, e.Provider, e.Config.Type, e.Config.Host)
	}
	for _, e := range reg.Databases {
		mustPrint("%s\t%s\t%s\t%s\t%s\n", e.Name, e.Provider, e.Config.Type, e.Config.Host, db.DefaultCloudRegistryPath)
	}
	if err := w.Flush(); err != nil {
		fatal(err)
	}
	if len(reg.Databases) == 0 && len(env) == 0 {
		fmt.Println("\nNo cloud databases registered yet. Get a free one at neon.tech (no card needed), then:")
		fmt.Println(`  go run ./cmd/registerdb my_db "postgresql://..."`)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
