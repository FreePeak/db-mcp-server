package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCloudRegression runs the standard regression battery against every
// free-tier cloud database that is either exported as an environment
// variable or present in the local cloud registry. With zero credentials it
// skips — no Docker containers are ever started.
func TestCloudRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cloud regression in short mode")
	}

	entries := ConfigsFromEnv()
	reg, err := LoadCloudRegistry(DefaultCloudRegistryPath)
	if err == nil {
		for _, e := range reg.Databases {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		t.Skip("No cloud databases configured: export NEON_DATABASE_URL (or any provider URL) or run 'go run ./cmd/registerdb'")
	}

	for _, entry := range entries {
		t.Run(entry.Name+"_"+entry.Provider, func(t *testing.T) {
			database, err := NewDatabase(entry.Config)
			require.NoError(t, err)

			err = database.Connect()
			if err != nil {
				// Free tiers scale to zero / pause; cold starts may refuse.
				t.Skipf("Cloud database %s not reachable (%v) — free tier may be asleep", entry.Name, err)
			}
			defer func() { _ = database.Close() }()

			ctx := context.Background()
			require.NoError(t, database.Ping(ctx), "Ping failed")

			testBasicQuery(t, database, entry.Config.Type)
			testExecuteOperations(t, database, entry.Config.Type)
			testTransactionSupport(t, database, entry.Config.Type)
			testDataTypeSupport(t, database, entry.Config.Type)
		})
	}
}
