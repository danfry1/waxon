package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds persistent waxon user configuration.
// Add new fields with `omitempty` so absent keys stay absent in the JSON —
// this keeps hand-edited config files clean and allows distinguishing
// "not set" from the zero value.
type Config struct {
	ClientID string `json:"client_id,omitempty"`

	// Theme is the name of a built-in palette (see `waxon themes`).
	Theme string `json:"theme,omitempty"`
	// Colors overrides individual palette entries on top of Theme, keyed by
	// palette field name (accent, bg, surface, text, text_sec, text_dim,
	// border, error, mode_search, mode_filter, overlay) with #RRGGBB values.
	Colors map[string]string `json:"colors,omitempty"`
	// Keys overrides keybindings, keyed by action name (up, down, next, …)
	// with a comma-separated list of keys, e.g. {"next": "n,right"}.
	Keys map[string]string `json:"keys,omitempty"`
	// Art picks how album art is drawn: "blocks" (default, Unicode
	// half-blocks), "kitty" (real images via the kitty graphics protocol,
	// for kitty and Ghostty), or "auto" (kitty when the terminal is known to
	// support it).
	Art string `json:"art,omitempty"`
	// Player runs a local Spotify Connect device through librespot, so waxon
	// plays without another Spotify client: "librespot" or "" / "off".
	Player string `json:"player,omitempty"`
	// PlayerPath is the librespot binary (default: librespot on PATH).
	PlayerPath string `json:"player_path,omitempty"`
	// MediaKeys, when false, stops waxon answering the keyboard's media keys
	// and showing in the system's Now Playing (default on).
	MediaKeys *bool `json:"media_keys,omitempty"`
}

// Dir returns the waxon config directory: $XDG_CONFIG_HOME/waxon when set,
// else ~/.config/waxon on Linux and macOS, and %AppData%\waxon on Windows
// (before 1.11, Windows used %USERPROFILE%\.config\waxon too; MigrateDir
// moves it, and until it has, that's still where config is read from).
func Dir() string {
	return dirFor(runtime.GOOS, os.Getenv, os.UserHomeDir, os.UserConfigDir, exists)
}

// legacyDir is where waxon kept its config on every platform before 1.11.
func legacyDir(getenv func(string) string, home func() (string, error)) string {
	h, err := home()
	if err != nil || h == "" {
		// os.UserHomeDir knows the platform convention ($HOME on Unix,
		// %USERPROFILE% on Windows); $HOME alone is empty on Windows and
		// would make the path relative to the working directory.
		h = getenv("HOME")
	}
	return filepath.Join(h, ".config", "waxon")
}

// dirFor is Dir with its environment passed in, for tests.
func dirFor(goos string, getenv func(string) string, home, userConfig func() (string, error), exists func(string) bool) string {
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "waxon")
	}
	legacy := legacyDir(getenv, home)
	if goos != "windows" {
		return legacy
	}
	appData, err := userConfig()
	if err != nil || appData == "" {
		return legacy
	}
	current := filepath.Join(appData, "waxon")
	if !exists(current) && exists(legacy) {
		return legacy // not moved yet (MigrateDir couldn't, or hasn't run)
	}
	return current
}

// MigrateDir moves a Windows config directory from its pre-1.11 place
// (%USERPROFILE%\.config\waxon) to %AppData%\waxon. It does nothing on
// other platforms, when $XDG_CONFIG_HOME is set, or once moved; if the move
// fails, waxon keeps using the old place. Call it once at startup.
func MigrateDir() error {
	return migrateDir(runtime.GOOS, os.Getenv, os.UserHomeDir, os.UserConfigDir, exists, os.Rename)
}

func migrateDir(goos string, getenv func(string) string, home, userConfig func() (string, error),
	exists func(string) bool, rename func(from, to string) error,
) error {
	if goos != "windows" || getenv("XDG_CONFIG_HOME") != "" {
		return nil
	}
	appData, err := userConfig()
	if err != nil || appData == "" {
		return nil //nolint:nilerr // no %AppData%: Dir keeps using the old place
	}
	from, to := legacyDir(getenv, home), filepath.Join(appData, "waxon")
	if !exists(from) || exists(to) {
		return nil
	}
	if err := os.MkdirAll(appData, 0o700); err != nil {
		return err
	}
	if err := rename(from, to); err != nil {
		return fmt.Errorf("moving waxon's config from %s to %s: %w", from, to, err)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DefaultPath returns the path to the config file (~/.config/waxon/config.json).
func DefaultPath() string {
	return filepath.Join(Dir(), "config.json")
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
	// An empty file (touch config.json), "null", or one saved by Windows
	// Notepad with a byte-order mark is still a config, just an empty one.
	data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	if len(data) == 0 || string(data) == "null" {
		return Config{}, nil
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return c, nil
}

func saveTo(path string, updates Config) error {
	// Load existing config so we don't clobber fields the caller didn't set.
	// A file that doesn't parse is the user's to fix: saving over it would
	// throw away everything in it.
	existing, err := loadFrom(path)
	if err != nil {
		return fmt.Errorf("%w (fix or delete %s, then try again)", err, path)
	}
	merged := merge(existing, updates)

	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := marshalKeepingUnknown(path, merged)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// marshalKeepingUnknown encodes cfg over the keys already in the file at
// path, so keys this version of waxon doesn't know (added by hand, or by a
// newer waxon) survive a save.
func marshalKeepingUnknown(path string, cfg Config) ([]byte, error) {
	out := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
		_ = json.Unmarshal(data, &out) // it parsed as a Config just now
	}
	if out == nil { // the file said null
		out = map[string]json.RawMessage{}
	}
	known, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(known, &fields); err != nil {
		return nil, err
	}
	for k, v := range fields {
		out[k] = v
	}
	return json.MarshalIndent(out, "", "  ")
}

// merge returns a Config where each zero-valued field in updates is
// filled from existing. Non-zero fields in updates always win.
func merge(existing, updates Config) Config {
	result := existing
	if updates.ClientID != "" {
		result.ClientID = updates.ClientID
	}
	if updates.Theme != "" {
		result.Theme = updates.Theme
	}
	if updates.Colors != nil {
		result.Colors = updates.Colors
	}
	if updates.Keys != nil {
		result.Keys = updates.Keys
	}
	if updates.Art != "" {
		result.Art = updates.Art
	}
	if updates.Player != "" {
		result.Player = updates.Player
	}
	if updates.PlayerPath != "" {
		result.PlayerPath = updates.PlayerPath
	}
	if updates.MediaKeys != nil {
		result.MediaKeys = updates.MediaKeys
	}
	return result
}
