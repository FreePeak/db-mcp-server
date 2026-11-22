package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdkAuth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/FreePeak/db-mcp-server/pkg/db"
)

// allDatabases is the databases value that grants every configured database.
const allDatabases = "*"

// AgentKey is one per-agent credential. Only the SHA-256 digest of the key is
// stored; the plaintext key lives with the agent. Databases is required: list
// database ids, or "*" for all of them.
type AgentKey struct {
	ID        string   `json:"id"`
	KeySHA256 string   `json:"key_sha256"`
	Scopes    []string `json:"scopes"`
	Databases []string `json:"databases"`
}

type agentKeyEntry struct {
	id        string
	digest    [sha256.Size]byte
	scopes    []string
	databases []string
}

// AgentKeyAuth authenticates Streamable HTTP requests against per-agent keys
// first and then, when legacyKey is set, the shared DB_MCP_API_KEY. Scope and
// database checks happen per tool call, so the middleware itself requires no
// scopes.
func AgentKeyAuth(keys []AgentKey, legacyKey string, legacyScopes []string) (func(http.Handler) http.Handler, error) {
	entries := make([]agentKeyEntry, 0, len(keys))
	for _, key := range keys {
		digest, err := hex.DecodeString(key.KeySHA256)
		if err != nil {
			return nil, err
		}
		entry := agentKeyEntry{id: key.ID, scopes: key.Scopes, databases: key.Databases}
		copy(entry.digest[:], digest)
		entries = append(entries, entry)
	}

	var legacy sdkAuth.TokenVerifier
	if legacyKey != "" {
		legacy = staticKeyVerifier(legacyKey, legacyScopes)
	}

	verifier := func(ctx context.Context, token string, r *http.Request) (*sdkAuth.TokenInfo, error) {
		digest := sha256.Sum256([]byte(token))
		var match *agentKeyEntry
		// Compare against every entry so the timing does not reveal which
		// position matched.
		for i := range entries {
			if subtle.ConstantTimeCompare(digest[:], entries[i].digest[:]) == 1 {
				match = &entries[i]
			}
		}
		if match != nil {
			return staticTokenInfo(match.id, match.scopes, map[string]any{"databases": match.databases}), nil
		}
		if legacy != nil {
			return legacy(ctx, token, r)
		}
		return nil, sdkAuth.ErrInvalidToken
	}
	return sdkAuth.RequireBearerToken(verifier, &sdkAuth.RequireBearerTokenOptions{}), nil
}

// LoadAgentKeys reads and validates the DB_MCP_AGENT_KEYS_FILE JSON list.
func LoadAgentKeys(path string) ([]AgentKey, error) {
	data, err := db.ResolveSecretValue("", "", path, "", "agent keys")
	if err != nil {
		return nil, err
	}
	var keys []AgentKey
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&keys); err != nil {
		return nil, fmt.Errorf("parse agent keys file: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("parse agent keys file: trailing data after the list")
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("agent keys file contains no entries")
	}
	if err := validateAgentKeys(keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// validateAgentKeys rejects entries that would silently grant nothing or
// make two agents share one identity. Errors name the entry id, never the
// key material.
func validateAgentKeys(keys []AgentKey) error {
	seenIDs := make(map[string]struct{}, len(keys))
	seenDigests := make(map[string]struct{}, len(keys))
	for i, key := range keys {
		if strings.TrimSpace(key.ID) == "" {
			return fmt.Errorf("agent key #%d: id is required", i+1)
		}
		if key.ID != strings.TrimSpace(key.ID) {
			return fmt.Errorf("agent key %q: id must not have leading or trailing whitespace", key.ID)
		}
		if _, dup := seenIDs[key.ID]; dup {
			return fmt.Errorf("agent key %q: duplicate id", key.ID)
		}
		seenIDs[key.ID] = struct{}{}

		digest, err := hex.DecodeString(key.KeySHA256)
		if err != nil || len(digest) != sha256.Size {
			return fmt.Errorf("agent key %q: key_sha256 must be 64 hex characters", key.ID)
		}
		emptyKeyDigest := sha256.Sum256(nil)
		if bytes.Equal(digest, emptyKeyDigest[:]) {
			return fmt.Errorf("agent key %q: key_sha256 is the SHA-256 of an empty key", key.ID)
		}
		if _, dup := seenDigests[string(digest)]; dup {
			return fmt.Errorf("agent key %q: duplicate key_sha256", key.ID)
		}
		seenDigests[string(digest)] = struct{}{}

		if len(key.Scopes) == 0 {
			return fmt.Errorf("agent key %q: at least one scope is required", key.ID)
		}
		for _, scope := range key.Scopes {
			if !isKnownScope(scope) {
				return fmt.Errorf("agent key %q: unknown scope %q", key.ID, scope)
			}
		}
		if len(key.Databases) == 0 {
			return fmt.Errorf(`agent key %q: databases is required (list database ids, or "*" for all)`, key.ID)
		}
	}
	return nil
}
