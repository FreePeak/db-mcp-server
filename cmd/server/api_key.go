package main

import (
	"fmt"
	"strings"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// Sources the Streamable HTTP API key can come from. At most one may be set.
const (
	apiKeySourceFlag = "-api-key"
	apiKeySourceEnv  = "DB_MCP_API_KEY"
	apiKeySourceFile = "DB_MCP_API_KEY_FILE"
)

// resolveAPIKey returns the API key from the single source that is set, and the
// name of that source. It returns an empty key and source when none is set; the
// caller decides whether that is fatal.
func resolveAPIKey(flagKey, envKey, keyFile string) (string, string, error) {
	var set []string
	if flagKey != "" {
		set = append(set, apiKeySourceFlag)
	}
	if envKey != "" {
		set = append(set, apiKeySourceEnv)
	}
	if keyFile != "" {
		set = append(set, apiKeySourceFile)
	}
	if len(set) > 1 {
		return "", "", fmt.Errorf("API key is set by multiple sources (%s); set exactly one", strings.Join(set, ", "))
	}
	if len(set) == 0 {
		return "", "", nil
	}

	var key string
	var err error
	switch set[0] {
	case apiKeySourceFlag:
		key = flagKey
	case apiKeySourceEnv:
		key = envKey
	default:
		key, err = db.ResolveSecretValue("", "", keyFile, "", "api_key")
		if err != nil {
			return "", "", err
		}
	}
	if strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("API key from %s is blank", set[0])
	}
	return key, set[0], nil
}
