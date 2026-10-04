// Package auth resolves the Hetzner Cloud token for the MCP server.
// Tools never accept a token. The server attaches it to every request.
package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Source names where the token was found. Useful in logs. Never contains the token.
const (
	SourceEnv     = "HCLOUD_TOKEN"
	SourceContext = "hcloud cli context"
)

type fileSchema struct {
	ActiveContext string `toml:"active_context"`
	Contexts      []struct {
		Name  string `toml:"name"`
		Token string `toml:"token"`
	} `toml:"contexts"`
}

// Resolve returns the project token the server will send as
// Authorization: Bearer. HCLOUD_TOKEN wins. Otherwise the active context in
// the hcloud CLI config (~/.config/hcloud/cli.toml) is used. HCLOUD_CONTEXT
// selects a context by name.
func Resolve() (token, source string, err error) {
	if token = os.Getenv("HCLOUD_TOKEN"); token != "" {
		return token, SourceEnv, nil
	}
	path := os.Getenv("HCLOUD_CONFIG")
	if path == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", "", fmt.Errorf("HCLOUD_TOKEN is unset and the home directory is unknown")
		}
		path = filepath.Join(home, ".config", "hcloud", "cli.toml")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", "", fmt.Errorf("HCLOUD_TOKEN is unset and %s does not exist", path)
		}
		return "", "", readErr
	}
	var schema fileSchema
	if err = toml.Unmarshal(data, &schema); err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	name := os.Getenv("HCLOUD_CONTEXT")
	if name == "" {
		name = schema.ActiveContext
	}
	for _, ctx := range schema.Contexts {
		if ctx.Name == name && ctx.Token != "" {
			return ctx.Token, SourceContext + " " + ctx.Name, nil
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("HCLOUD_TOKEN is unset and %s has no active context", path)
	}
	return "", "", fmt.Errorf("HCLOUD_TOKEN is unset and context %q has no token", name)
}
