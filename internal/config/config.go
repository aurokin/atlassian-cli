// Package config defines the on-disk CLI configuration schema and a file
// store for it. It records site profiles but never raw secrets: tokens are
// referenced indirectly (see SiteProfile.TokenRef).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aurokin/atlassian-cli/internal/apperr"
)

// CurrentVersion is the config schema version written by this CLI.
const CurrentVersion = 1

const (
	configDirName       = "atlassian-cli"
	configFileName      = "config.json"
	credentialsFileName = "credentials.json"
)

// Config is the root configuration document.
type Config struct {
	Version int                    `json:"version"`
	Sites   map[string]SiteProfile `json:"sites"`
	// Aliases maps a user-defined command name to its expansion (a shell-style
	// argument string). It is shared by every atl-* binary: each expands aliases
	// against this map before dispatch (see internal/cli).
	Aliases map[string]string `json:"aliases,omitempty"`
}

// SiteProfile describes one configured Atlassian site for one product.
//
// TokenRef holds an indirect reference to a credential (for example an
// environment variable name), never a raw token value.
type SiteProfile struct {
	Product    string `json:"product"`
	Deployment string `json:"deployment"`
	BaseURL    string `json:"base_url"`
	APIBaseURL string `json:"api_base_url,omitempty"`
	CloudID    string `json:"cloud_id,omitempty"`
	Username   string `json:"username,omitempty"`
	TokenStyle string `json:"token_style"`
	AuthType   string `json:"auth_type"`
	TokenRef   string `json:"token_ref,omitempty"`
}

// New returns an empty config at the current schema version.
func New() Config {
	return Config{
		Version: CurrentVersion,
		Sites:   map[string]SiteProfile{},
		Aliases: map[string]string{},
	}
}

// dir returns the CLI configuration directory, honoring XDG_CONFIG_HOME and
// otherwise using ~/.config.
func dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, configDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locate home directory: %w", err)
	}
	return filepath.Join(home, ".config", configDirName), nil
}

// DefaultPath returns the config file path under the user's config directory.
func DefaultPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, configFileName), nil
}

// CredentialsPath returns the path of the 0600 fallback credentials file,
// alongside the main config file. It is used only when no OS keychain is
// available; config.json itself never holds a raw token.
func CredentialsPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, credentialsFileName), nil
}

// Load reads the config at path. A missing file yields an empty config and no
// error; a malformed file yields a structured *apperr.Error.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, apperr.New("invalid_config",
			fmt.Sprintf("config file %s is not valid JSON: %v", path, err))
	}
	if c.Sites == nil {
		c.Sites = map[string]SiteProfile{}
	}
	if c.Aliases == nil {
		c.Aliases = map[string]string{}
	}
	return c, nil
}

// Save writes c to path as indented JSON, creating parent directories as
// needed. The directory is created with 0700 and the file with 0600 so token
// references stay private to the user.
//
// The write is atomic: data is written to a temporary file in the same
// directory and renamed over path. A crash or concurrent run can never
// observe a partial file, and the rename replaces a symlink at path rather
// than following it to some other location.
func Save(path string, c Config) error {
	if c.Sites == nil {
		c.Sites = map[string]SiteProfile{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf("config: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("config: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: replace %s: %w", path, err)
	}
	return nil
}
