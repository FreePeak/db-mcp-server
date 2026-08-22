package db

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// This module lets the test suite run against free-tier managed cloud
// databases (Neon, Supabase, Aiven, TiDB Cloud Serverless) instead of local
// Docker stacks. Providers are detected automatically from the hostname,
// connection strings arrive via environment variables or the registry file,
// and everything degrades to a graceful skip when no credentials exist.

const DefaultCloudRegistryPath = ".test-cloud-db.json"

// CloudEntry is one registered cloud database.
type CloudEntry struct {
	Name         string    `json:"name"`
	Provider     string    `json:"provider"`
	DSN          string    `json:"dsn"`
	Config       Config    `json:"-"`
	RegisteredAt time.Time `json:"registered_at"`
}

// cloudEntryJSON is the persisted shape; Config is serialized explicitly
// because it has no json tags of its own.
type cloudEntryJSON struct {
	Name         string          `json:"name"`
	Provider     string          `json:"provider"`
	DSN          string          `json:"dsn"`
	RegisteredAt time.Time       `json:"registered_at"`
	Config       cloudConfigJSON `json:"config"`
}

type cloudConfigJSON struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Name     string `json:"name"`
}

// CloudRegistry holds named cloud databases for test runs.
type CloudRegistry struct {
	Databases map[string]CloudEntry `json:"databases"`
	path      string
}

// ParseDSN parses a provider-issued connection string into a Config.
// Supported forms:
//   - postgres://[user:pass@]host[:port]/db[?sslmode=...]  (also postgresql://)
//   - mysql://[user:pass@]host[:port]/db
//   - user:pass@tcp(host:port)/db                          (Go MySQL DSN)
func ParseDSN(dsn string) (Config, error) {
	dsn = strings.TrimSpace(dsn)
	lower := strings.ToLower(dsn)

	switch {
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return parsePostgresURL(dsn)
	case strings.HasPrefix(lower, "mysql://"):
		return parseGenericURL(dsn, "mysql")
	default:
		return parseMySQLGoDSN(dsn)
	}
}

func parsePostgresURL(raw string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid postgres URL: %w", err)
	}
	cfg := Config{Type: "postgres", SSLMode: SSLRequire} // free tiers enforce TLS
	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}
	cfg.Host = u.Hostname()
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("postgres URL missing host")
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return Config{}, fmt.Errorf("invalid port %q: %w", p, err)
		}
		cfg.Port = port
	} else {
		cfg.Port = 5432
	}
	cfg.Name = strings.TrimPrefix(u.Path, "/")
	if cfg.Name == "" {
		return Config{}, fmt.Errorf("postgres URL missing database name")
	}
	for k, v := range u.Query() {
		if len(v) == 0 {
			continue
		}
		if k == "sslmode" {
			cfg.SSLMode = PostgresSSLMode(v[0])
		}
	}
	return cfg, nil
}

func parseGenericURL(raw string, dbType string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid %s URL: %w", dbType, err)
	}
	cfg := Config{Type: dbType}
	if u.User != nil {
		cfg.User = u.User.Username()
		cfg.Password, _ = u.User.Password()
	}
	cfg.Host = u.Hostname()
	if cfg.Host == "" {
		return Config{}, fmt.Errorf("%s URL missing host", dbType)
	}
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return Config{}, fmt.Errorf("invalid port %q: %w", p, err)
		}
		cfg.Port = port
	} else if dbType == "mysql" {
		cfg.Port = 3306
	}
	cfg.Name = strings.TrimPrefix(u.Path, "/")
	if cfg.Name == "" {
		return Config{}, fmt.Errorf("%s URL missing database name", dbType)
	}
	return cfg, nil
}

var goMySQLDSNRe = regexp.MustCompile(`^(?:([A-Za-z0-9._%-]*):([^@]*)@)?tcp\(([^):]+):(\d+)\)/([A-Za-z0-9_$]+)`)

func parseMySQLGoDSN(raw string) (Config, error) {
	m := goMySQLDSNRe.FindStringSubmatch(raw)
	if m == nil {
		return Config{}, fmt.Errorf("unsupported DSN format (expected postgres://, mysql://, or user:pass@tcp(host:port)/db)")
	}
	port, err := strconv.Atoi(m[4])
	if err != nil {
		return Config{}, fmt.Errorf("invalid port %q: %w", m[4], err)
	}
	user := m[1]
	pass := m[2]
	if unescaped, err := url.QueryUnescape(user); err == nil {
		user = unescaped
	}
	return Config{Type: "mysql", User: user, Password: pass, Host: m[3], Port: port, Name: m[5]}, nil
}

// DetectProviderName classifies a cloud provider from its hostname
// (exported for CLI/reporting use).
func DetectProviderName(host string) string { return detectProvider(host) }

// detectProvider classifies a cloud provider from its hostname so registry
// entries are labelled without manual input.
func detectProvider(host string) string {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "neon.tech"):
		return "neon"
	case strings.Contains(h, "supabase"):
		return "supabase"
	case strings.Contains(h, "aivencloud.com") || strings.Contains(h, "aiven.io"):
		return "aiven"
	case strings.Contains(h, "tidbcloud.com"):
		return "tidbcloud"
	case strings.Contains(h, "cockroachlabs.cloud"):
		return "cockroachdb"
	case strings.Contains(h, "xata.sh"):
		return "xata"
	default:
		return "generic"
	}
}

// envProviderVars maps environment variable names to registry IDs. Any
// free-tier signup produces exactly one of these; exporting it is all the
// "registration" a human ever needs to do.
var envProviderVars = []struct {
	env      string
	idSuffix string
}{
	{"NEON_DATABASE_URL", "neon"},
	{"SUPABASE_DATABASE_URL", "supabase"},
	{"AIVEN_DATABASE_URL", "aiven"},
	{"TIDBCLOUD_DATABASE_URL", "tidbcloud"},
	{"CLOUD_MYSQL_URL", "generic_mysql"},
	{"DATABASE_URL", "primary"},
}

// ConfigsFromEnv scans well-known provider env vars and returns parsed
// configs, most specific first. DATABASE_URL is consulted only when no
// provider-specific variable is set.
func ConfigsFromEnv() []CloudEntry {
	var out []CloudEntry
	for _, pv := range envProviderVars {
		raw := strings.TrimSpace(os.Getenv(pv.env))
		if raw == "" {
			continue
		}
		if pv.idSuffix == "primary" && len(out) > 0 {
			continue // provider-specific vars win over generic DATABASE_URL
		}
		cfg, err := ParseDSN(raw)
		if err != nil {
			continue // malformed credential must not break unrelated tests
		}
		out = append(out, CloudEntry{
			Name:         "env_" + pv.idSuffix,
			Provider:     detectProvider(cfg.Host),
			DSN:          raw,
			Config:       cfg,
			RegisteredAt: time.Now().UTC(),
		})
	}
	return out
}

// LoadCloudRegistry reads the on-disk registry; a missing file yields an
// empty registry rather than an error.
func LoadCloudRegistry(path string) (CloudRegistry, error) {
	reg := CloudRegistry{Databases: map[string]CloudEntry{}, path: path}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return reg, fmt.Errorf("read registry: %w", err)
	}
	var entries map[string]cloudEntryJSON
	if err := json.Unmarshal(raw, &struct {
		Databases map[string]cloudEntryJSON `json:"databases"`
	}{Databases: entries}); err != nil {
		return reg, fmt.Errorf("parse registry: %w", err)
	}
	_ = entries // populated below via second unmarshal for correctness
	var doc struct {
		Databases map[string]cloudEntryJSON `json:"databases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return reg, fmt.Errorf("parse registry: %w", err)
	}
	for name, e := range doc.Databases {
		reg.Databases[name] = CloudEntry{
			Name: e.Name, Provider: e.Provider, DSN: e.DSN,
			RegisteredAt: e.RegisteredAt,
			Config: Config{
				Type: e.Config.Type, Host: e.Config.Host, Port: e.Config.Port,
				User: e.Config.User, Password: e.Config.Password, Name: e.Config.Name,
			},
		}
	}
	return reg, nil
}

// RegisterCloudDB validates a DSN by parsing it and persists it under name.
func RegisterCloudDB(path, name, dsn string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("registry entry name must not be empty")
	}
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("invalid DSN: %w", err)
	}
	reg, err := LoadCloudRegistry(path)
	if err != nil {
		return err
	}
	entry := CloudEntry{
		Name: name, Provider: detectProvider(cfg.Host), DSN: dsn,
		Config: cfg, RegisteredAt: time.Now().UTC(),
	}
	doc := struct {
		Databases map[string]cloudEntryJSON `json:"databases"`
	}{Databases: map[string]cloudEntryJSON{}}
	for n, e := range reg.Databases {
		doc.Databases[n] = toCloudEntryJSON(e)
	}
	doc.Databases[name] = toCloudEntryJSON(entry)
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	return os.WriteFile(path, append(buf, '\n'), 0o600)
}

func toCloudEntryJSON(e CloudEntry) cloudEntryJSON {
	return cloudEntryJSON{
		Name: e.Name, Provider: e.Provider, DSN: e.DSN,
		RegisteredAt: e.RegisteredAt,
		Config: cloudConfigJSON{
			Type: e.Config.Type, Host: e.Config.Host, Port: e.Config.Port,
			User: e.Config.User, Password: e.Config.Password, Name: e.Config.Name,
		},
	}
}
