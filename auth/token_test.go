package auth

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSaveAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	tok := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}

	if loaded.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, tok.AccessToken)
	}
	if loaded.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, tok.RefreshToken)
	}
}

func TestLoadTokenMissing(t *testing.T) {
	_, err := LoadToken("/nonexistent/path/token.json")
	if err == nil {
		t.Fatal("expected error for missing token file")
	}
}

func TestDefaultTokenPath(t *testing.T) {
	path := DefaultTokenPath()
	if path == "" {
		t.Fatal("DefaultTokenPath returned empty string")
	}
	if filepath.Base(path) != "token.json" {
		t.Errorf("expected token.json, got %s", filepath.Base(path))
	}
}
