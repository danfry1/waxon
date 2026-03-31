package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds persistent waxon user configuration.
// Add new fields with `omitempty` so absent keys stay absent in the JSON —
// this keeps hand-edited config files clean and allows distinguishing
// "not set" from the zero value.
type Config struct {
	ClientID string `json:"client_id,omitempty"`
}

// DefaultPath returns the path to the config file (~/.config/waxon/config.json).
func DefaultPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	return filepath.Join(configDir, "waxon", "config.json")
}

// Load reads the config from disk. Returns a zero Config (not an error)
// if the file does not exist.
func Load() (Config, error) {
	return loadFrom(DefaultPath())
}

// Save merges cfg into the existing config on disk and writes it back.
// Fields that are zero-valued in cfg are left unchanged on disk, so
// callers only need to set the fields they want to update.
func Save(cfg Config) error {
	return saveTo(DefaultPath(), cfg)
}

func loadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return c, nil
}

func saveTo(path string, updates Config) error {
	// Load existing config so we don't clobber fields the caller didn't set.
	existing, _ := loadFrom(path)
	merged := merge(existing, updates)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// merge returns a Config where each zero-valued field in updates is
// filled from existing. Non-zero fields in updates always win.
func merge(existing, updates Config) Config {
	result := existing
	if updates.ClientID != "" {
		result.ClientID = updates.ClientID
	}
	return result
}
