package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Config{ClientID: "test-client-id-123"}
	if err := saveTo(path, cfg); err != nil {
		t.Fatalf("saveTo: %v", err)
	}

	loaded, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if loaded.ClientID != cfg.ClientID {
		t.Errorf("ClientID = %q, want %q", loaded.ClientID, cfg.ClientID)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "config.json")

	cfg := Config{ClientID: "nested-test-id"}
	if err := saveTo(path, cfg); err != nil {
		t.Fatalf("saveTo: %v", err)
	}

	loaded, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if loaded.ClientID != cfg.ClientID {
		t.Errorf("ClientID = %q, want %q", loaded.ClientID, cfg.ClientID)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := loadFrom("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected no error for missing config, got: %v", err)
	}
	if cfg.ClientID != "" {
		t.Errorf("expected empty ClientID, got %q", cfg.ClientID)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{not-valid-json}"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := loadFrom(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadEmptyObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if cfg.ClientID != "" {
		t.Errorf("expected empty ClientID from empty JSON object, got %q", cfg.ClientID)
	}
}

func TestLoadExtraFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := []byte(`{"client_id":"my-id","unknown_field":"value","nested":{"a":1}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if cfg.ClientID != "my-id" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "my-id")
	}
}

func TestLoadPermissionError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte(`{"client_id":"x"}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(path, 0o600)

	_, err := loadFrom(path)
	if err == nil {
		t.Fatal("expected error for unreadable config file")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Fatal("DefaultPath returned empty string")
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("expected config.json, got %s", filepath.Base(path))
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
}

func TestDefaultPathContainsWaxon(t *testing.T) {
	path := DefaultPath()
	found := false
	dir := path
	for dir != filepath.Dir(dir) {
		if filepath.Base(dir) == "waxon" {
			found = true
			break
		}
		dir = filepath.Dir(dir)
	}
	if !found {
		t.Errorf("expected path to contain 'waxon', got %q", path)
	}
}

func TestConfigZeroValue(t *testing.T) {
	var cfg Config
	if cfg.ClientID != "" {
		t.Errorf("zero Config.ClientID = %q, want empty", cfg.ClientID)
	}
}

// TestSaveMergesExistingFields verifies that saving a partial config
// does not clobber fields already on disk.
func TestSaveMergesExistingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write initial config with client_id.
	if err := saveTo(path, Config{ClientID: "original-id"}); err != nil {
		t.Fatalf("saveTo (initial): %v", err)
	}

	// Save an empty Config — should preserve the existing client_id.
	if err := saveTo(path, Config{}); err != nil {
		t.Fatalf("saveTo (empty): %v", err)
	}

	loaded, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if loaded.ClientID != "original-id" {
		t.Errorf("ClientID = %q, want %q (should not be clobbered)", loaded.ClientID, "original-id")
	}
}

// TestSaveOverwritesField verifies that a non-zero field in updates wins.
func TestSaveOverwritesField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := saveTo(path, Config{ClientID: "old-id"}); err != nil {
		t.Fatalf("saveTo (initial): %v", err)
	}

	if err := saveTo(path, Config{ClientID: "new-id"}); err != nil {
		t.Fatalf("saveTo (update): %v", err)
	}

	loaded, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	if loaded.ClientID != "new-id" {
		t.Errorf("ClientID = %q, want %q", loaded.ClientID, "new-id")
	}
}

func TestMerge(t *testing.T) {
	existing := Config{ClientID: "existing-id"}

	// Zero update preserves existing.
	got := merge(existing, Config{})
	if got.ClientID != "existing-id" {
		t.Errorf("merge with zero update: ClientID = %q, want %q", got.ClientID, "existing-id")
	}

	// Non-zero update overwrites.
	got = merge(existing, Config{ClientID: "new-id"})
	if got.ClientID != "new-id" {
		t.Errorf("merge with update: ClientID = %q, want %q", got.ClientID, "new-id")
	}
}
