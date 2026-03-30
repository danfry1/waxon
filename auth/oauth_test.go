package auth

import (
	"testing"
)

func TestSpotifyOAuthConfig(t *testing.T) {
	cfg := SpotifyOAuthConfig("test-client-id", "http://localhost:8080/callback")

	if cfg.ClientID != "test-client-id" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "test-client-id")
	}
	if cfg.RedirectURL != "http://localhost:8080/callback" {
		t.Errorf("RedirectURL = %q", cfg.RedirectURL)
	}
	if cfg.Endpoint.AuthURL != "https://accounts.spotify.com/authorize" {
		t.Errorf("AuthURL = %q", cfg.Endpoint.AuthURL)
	}
	if len(cfg.Scopes) == 0 {
		t.Error("expected scopes to be set")
	}
}

func TestGenerateVerifier(t *testing.T) {
	v := generateVerifier()
	if len(v) < 43 || len(v) > 128 {
		t.Errorf("verifier length = %d, expected 43-128", len(v))
	}
}

func TestChallengeFromVerifier(t *testing.T) {
	v := generateVerifier()
	c := challengeFromVerifier(v)
	if c == "" {
		t.Error("challenge should not be empty")
	}
	if c == v {
		t.Error("challenge should differ from verifier")
	}
}
