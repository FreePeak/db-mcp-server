package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestParsePostgresDSN covers URL-form Postgres connection strings as emitted
// by every free cloud provider (Neon, Supabase, Aiven, Render).
func TestParsePostgresDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    Config
		wantErr bool
	}{
		{
			name: "neon_style",
			dsn:  "postgresql://alice:s3cret@ep-cool-name-123.us-east-2.aws.neon.tech/neondb?sslmode=require",
			want: Config{
				Type: "postgres", User: "alice", Password: "s3cret",
				Host: "ep-cool-name-123.us-east-2.aws.neon.tech", Port: 5432,
				Name: "neondb", SSLMode: "require",
			},
		},
		{
			name: "supabase_pooler",
			dsn:  "postgres://postgres.xyz:pw@aws-0-us-east-1.pooler.supabase.com:6543/postgres",
			want: Config{
				Type: "postgres", User: "postgres.xyz", Password: "pw",
				Host: "aws-0-us-east-1.pooler.supabase.com", Port: 6543,
				Name: "postgres", SSLMode: "require",
			},
		},
		{
			name:    "bad_scheme",
			dsn:     "http://example.com/db",
			wantErr: true,
		},
		{
			name:    "no_host",
			dsn:     "postgres:///db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.dsn)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDSN(%q) failed: %v", tt.dsn, err)
			}
			if got.Type != tt.want.Type || got.Host != tt.want.Host ||
				got.Port != tt.want.Port || got.User != tt.want.User ||
				got.Password != tt.want.Password || got.Name != tt.want.Name ||
				string(got.SSLMode) != string(tt.want.SSLMode) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestParseMySQLDSN covers both mysql:// URLs and Go DSN form used by TiDB
// Cloud serverless / PlanetScale-style endpoints.
func TestParseMySQLDSN(t *testing.T) {
	t.Run("url_form", func(t *testing.T) {
		got, err := ParseDSN("mysql://root:pw@gateway01.us-east-1.prod.shared.aws.tidbcloud.com:4000/test")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if got.Type != "mysql" || got.Host != "gateway01.us-east-1.prod.shared.aws.tidbcloud.com" ||
			got.Port != 4000 || got.User != "root" || got.Password != "pw" || got.Name != "test" {
			t.Fatalf("unexpected config: %+v", got)
		}
	})

	t.Run("go_dsn_form", func(t *testing.T) {
		got, err := ParseDSN("root:pw@tcp(aiven-free.db.aivencloud.com:12345)/defaultdb")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if got.Type != "mysql" || got.Host != "aiven-free.db.aivencloud.com" ||
			got.Port != 12345 || got.User != "root" || got.Name != "defaultdb" {
			t.Fatalf("unexpected config: %+v", got)
		}
	})
}

// TestCloudRegistryRoundTrip proves register → load persistence.
func TestCloudRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud-dbs.json")

	reg, err := LoadCloudRegistry(path)
	if err != nil {
		t.Fatalf("load empty registry failed: %v", err)
	}
	if len(reg.Databases) != 0 {
		t.Fatalf("expected empty registry, got %d", len(reg.Databases))
	}

	if err := RegisterCloudDB(path, "neon_ci", "postgresql://u:p@h.neon.tech/db?sslmode=require"); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	reg, err = LoadCloudRegistry(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	entry, ok := reg.Databases["neon_ci"]
	if !ok {
		t.Fatalf("neon_ci not registered; registry=%+v", reg.Databases)
	}
	if entry.Config.Host != "h.neon.tech" || entry.Provider != "neon" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

// TestDetectProvider verifies provider classification from hostname so
// registrations are labelled automatically.
func TestDetectProvider(t *testing.T) {
	cases := map[string]string{
		"ep-cool-123.us-east-2.aws.neon.tech":               "neon",
		"aws-0-us-east-1.pooler.supabase.com":               "supabase",
		"myfree.db.aivencloud.com":                          "aiven",
		"gateway01.us-east-1.prod.shared.aws.tidbcloud.com": "tidbcloud",
		"localhost": "generic",
	}
	for host, want := range cases {
		if got := detectProvider(host); got != want {
			t.Errorf("detectProvider(%q) = %q, want %q", host, got, want)
		}
	}
}

// TestConfigFromEnv proves the harness picks up provider env vars without
// any manual registration step — drop NEON_DATABASE_URL in the environment
// and the cloud regression suite runs against it.
func TestConfigFromEnv(t *testing.T) {
	t.Setenv("NEON_DATABASE_URL", "postgresql://u:p@ep-x.neon.tech/dbx?sslmode=require")

	configs := ConfigsFromEnv()
	if len(configs) != 1 {
		t.Fatalf("expected 1 env config, got %d", len(configs))
	}
	if configs[0].Name != "env_neon" || configs[0].Config.Host != "ep-x.neon.tech" {
		t.Fatalf("unexpected config: %+v", configs[0])
	}
}

// TestConfigsFromEnvEmpty ensures silence (not error) with no credentials.
func TestConfigsFromEnvEmpty(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "NEON_DATABASE_URL", "SUPABASE_DATABASE_URL", "CLOUD_MYSQL_URL"} {
		os.Unsetenv(k)
	}
	if configs := ConfigsFromEnv(); len(configs) != 0 {
		t.Fatalf("expected zero configs, got %+v", configs)
	}
}

// TestRegistryJSONShape locks in the on-disk format for portability.
func TestRegistryJSONShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reg.json")
	if err := RegisterCloudDB(path, "aiven_mysql", "mysql://u:p@host.aivencloud.com:1234/db"); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := doc["databases"]; !ok {
		t.Fatalf("missing 'databases' key: %s", raw)
	}
}
