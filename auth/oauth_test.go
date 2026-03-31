package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
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
	v, err := generateVerifier()
	if err != nil {
		t.Fatalf("generateVerifier error: %v", err)
	}
	if len(v) < 43 || len(v) > 128 {
		t.Errorf("verifier length = %d, expected 43-128", len(v))
	}
}

func TestChallengeFromVerifier(t *testing.T) {
	v := mustVerifier(t)
	c := challengeFromVerifier(v)
	if c == "" {
		t.Error("challenge should not be empty")
	}
	if c == v {
		t.Error("challenge should differ from verifier")
	}
}

func TestSpotifyOAuthConfigScopes(t *testing.T) {
	cfg := SpotifyOAuthConfig("test-id", "http://localhost/callback")

	required := []string{
		"user-read-playback-state",
		"user-modify-playback-state",
	}

	for _, scope := range required {
		found := false
		for _, s := range cfg.Scopes {
			if s == scope {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required scope %q", scope)
		}
	}
}

func TestGenerateVerifierUniqueness(t *testing.T) {
	v1 := mustVerifier(t)
	v2 := mustVerifier(t)
	if v1 == v2 {
		t.Error("two generated verifiers should not be identical")
	}
}

func TestChallengeFromVerifierDeterministic(t *testing.T) {
	v := mustVerifier(t)
	c1 := challengeFromVerifier(v)
	c2 := challengeFromVerifier(v)
	if c1 != c2 {
		t.Errorf("same verifier produced different challenges: %q vs %q", c1, c2)
	}
}

func TestChallengeFromVerifierDiffers(t *testing.T) {
	v1 := mustVerifier(t)
	v2 := mustVerifier(t)
	c1 := challengeFromVerifier(v1)
	c2 := challengeFromVerifier(v2)
	if c1 == c2 {
		t.Error("different verifiers should produce different challenges")
	}
}

func TestSpotifyOAuthConfigTokenURL(t *testing.T) {
	cfg := SpotifyOAuthConfig("test-id", "http://localhost/callback")
	expected := "https://accounts.spotify.com/api/token"
	if cfg.Endpoint.TokenURL != expected {
		t.Errorf("TokenURL = %q, want %q", cfg.Endpoint.TokenURL, expected)
	}
}

func TestSpotifyOAuthConfigAuthURL(t *testing.T) {
	cfg := SpotifyOAuthConfig("my-client", "http://localhost:9999/cb")
	expected := "https://accounts.spotify.com/authorize"
	if cfg.Endpoint.AuthURL != expected {
		t.Errorf("AuthURL = %q, want %q", cfg.Endpoint.AuthURL, expected)
	}
}

func TestSpotifyOAuthConfigAllScopes(t *testing.T) {
	cfg := SpotifyOAuthConfig("id", "http://localhost/cb")

	allExpected := []string{
		"user-read-playback-state",
		"user-modify-playback-state",
		"user-read-currently-playing",
		"playlist-read-private",
		"playlist-read-collaborative",
		"user-library-read",
		"user-read-recently-played",
	}

	if len(cfg.Scopes) != len(allExpected) {
		t.Fatalf("got %d scopes, want %d", len(cfg.Scopes), len(allExpected))
	}

	for i, scope := range allExpected {
		if cfg.Scopes[i] != scope {
			t.Errorf("scope[%d] = %q, want %q", i, cfg.Scopes[i], scope)
		}
	}
}

func TestGenerateVerifierLength(t *testing.T) {
	v := mustVerifier(t)
	// 64 bytes base64-raw-url-encoded = 86 characters
	if len(v) != 86 {
		t.Errorf("verifier length = %d, want 86", len(v))
	}
}

func TestGenerateVerifierBase64URLEncoded(t *testing.T) {
	v := mustVerifier(t)
	// Should decode without error using base64 raw URL encoding.
	_, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		t.Errorf("verifier is not valid base64 raw URL encoding: %v", err)
	}
}

func TestChallengeFromVerifierBase64URLEncoded(t *testing.T) {
	v := mustVerifier(t)
	c := challengeFromVerifier(v)
	_, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		t.Errorf("challenge is not valid base64 raw URL encoding: %v", err)
	}
}

func TestChallengeFromVerifierLength(t *testing.T) {
	v := mustVerifier(t)
	c := challengeFromVerifier(v)
	// SHA-256 is 32 bytes; base64-raw-url-encoded = 43 characters
	if len(c) != 43 {
		t.Errorf("challenge length = %d, want 43", len(c))
	}
}

func TestDefaultClientID(t *testing.T) {
	if DefaultClientID == "" {
		t.Error("DefaultClientID should not be empty")
	}
	if len(DefaultClientID) != 32 {
		t.Errorf("DefaultClientID length = %d, want 32", len(DefaultClientID))
	}
}

// TestCallbackHandler_ValidCode tests the extracted callbackHandler to verify
// it extracts the authorization code correctly when state matches.
func TestCallbackHandler_ValidCode(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("test-state-123", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/login?state=test-state-123&code=auth-code-xyz")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case code := <-codeCh:
		if code != "auth-code-xyz" {
			t.Errorf("code = %q, want %q", code, "auth-code-xyz")
		}
	default:
		t.Error("expected code on channel")
	}
}

// TestCallbackHandler_StateMismatch verifies the handler rejects requests
// with a mismatched state parameter.
func TestCallbackHandler_StateMismatch(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("correct-state", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/login?state=wrong-state&code=abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	select {
	case authErr := <-errCh:
		if !strings.Contains(authErr.Error(), "state mismatch") {
			t.Errorf("error = %q, want state mismatch message", authErr)
		}
	default:
		t.Error("expected error on channel")
	}
}

// TestCallbackHandler_AuthDenied verifies the handler reports auth denial
// when no code is present.
func TestCallbackHandler_AuthDenied(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("my-state", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/login?state=my-state&error=access_denied")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	select {
	case authErr := <-errCh:
		if !strings.Contains(authErr.Error(), "access_denied") {
			t.Errorf("error = %q, want to contain 'access_denied'", authErr)
		}
	default:
		t.Error("expected error on channel")
	}
}

// ---------------------------------------------------------------------------
// callbackHandler — non-/login path returns 404
// ---------------------------------------------------------------------------

func TestCallbackHandler_WrongPath(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("state", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/wrong")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	// Neither channel should have a value
	select {
	case <-codeCh:
		t.Error("unexpected code on channel")
	default:
	}
	select {
	case <-errCh:
		t.Error("unexpected error on channel")
	default:
	}
}

// ---------------------------------------------------------------------------
// callbackHandler — empty error param
// ---------------------------------------------------------------------------

func TestCallbackHandler_EmptyError(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("state", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	// state matches, but no code and no error param
	resp, err := http.Get(server.URL + "/login?state=state")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	select {
	case authErr := <-errCh:
		if !strings.Contains(authErr.Error(), "auth denied") {
			t.Errorf("error = %q, want to contain 'auth denied'", authErr)
		}
	default:
		t.Error("expected error on channel")
	}
}

// ---------------------------------------------------------------------------
// SpotifyOAuthConfig — different client IDs
// ---------------------------------------------------------------------------

func TestSpotifyOAuthConfigDifferentIDs(t *testing.T) {
	cfg1 := SpotifyOAuthConfig("id-one", "http://localhost:1234/cb")
	cfg2 := SpotifyOAuthConfig("id-two", "http://localhost:5678/cb")

	if cfg1.ClientID == cfg2.ClientID {
		t.Error("configs with different client IDs should differ")
	}
	if cfg1.RedirectURL == cfg2.RedirectURL {
		t.Error("configs with different redirect URLs should differ")
	}
}

// ---------------------------------------------------------------------------
// Verifier/challenge PKCE relationship
// ---------------------------------------------------------------------------

func TestPKCEFlowVerifierChallengePair(t *testing.T) {
	// Generate multiple pairs and verify each challenge is deterministic
	for i := 0; i < 5; i++ {
		v := mustVerifier(t)
		c1 := challengeFromVerifier(v)
		c2 := challengeFromVerifier(v)
		if c1 != c2 {
			t.Errorf("iteration %d: challenge not deterministic for same verifier", i)
		}
	}
}

// ---------------------------------------------------------------------------
// spotifyScopes package-level variable
// ---------------------------------------------------------------------------

func TestSpotifyScopesNotEmpty(t *testing.T) {
	cfg := SpotifyOAuthConfig("test", "http://localhost/cb")
	if len(cfg.Scopes) < 5 {
		t.Errorf("expected at least 5 scopes, got %d", len(cfg.Scopes))
	}
}

// ---------------------------------------------------------------------------
// SpotifyOAuthConfig generates valid AuthCodeURL
// ---------------------------------------------------------------------------

func TestSpotifyOAuthConfigAuthCodeURL(t *testing.T) {
	cfg := SpotifyOAuthConfig("my-client", "http://localhost:9999/login")
	verifier := mustVerifier(t)
	challenge := challengeFromVerifier(verifier)

	url := cfg.AuthCodeURL("test-state",
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
	)

	if !strings.Contains(url, "accounts.spotify.com/authorize") {
		t.Errorf("auth URL missing Spotify authorize endpoint: %s", url)
	}
	if !strings.Contains(url, "client_id=my-client") {
		t.Errorf("auth URL missing client_id: %s", url)
	}
	if !strings.Contains(url, "state=test-state") {
		t.Errorf("auth URL missing state: %s", url)
	}
	if !strings.Contains(url, "code_challenge_method=S256") {
		t.Errorf("auth URL missing code_challenge_method: %s", url)
	}
	if !strings.Contains(url, "code_challenge=") {
		t.Errorf("auth URL missing code_challenge: %s", url)
	}
	if !strings.Contains(url, "redirect_uri=") {
		t.Errorf("auth URL missing redirect_uri: %s", url)
	}
}

// ---------------------------------------------------------------------------
// callbackHandler — multiple requests (only first code wins)
// ---------------------------------------------------------------------------

func TestCallbackHandler_OnlyFirstCodeWins(t *testing.T) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	handler := callbackHandler("s", codeCh, errCh)
	server := httptest.NewServer(handler)
	defer server.Close()

	// First request: should succeed
	resp1, err := http.Get(server.URL + "/login?state=s&code=first")
	if err != nil {
		t.Fatalf("GET 1: %v", err)
	}
	resp1.Body.Close()

	// Second request: code channel is already full (buffered 1)
	resp2, err := http.Get(server.URL + "/login?state=s&code=second")
	if err != nil {
		t.Fatalf("GET 2: %v", err)
	}
	resp2.Body.Close()

	code := <-codeCh
	if code != "first" {
		t.Errorf("code = %q, want %q", code, "first")
	}

	// Channel should be empty now
	select {
	case extra := <-codeCh:
		t.Errorf("unexpected extra code: %q", extra)
	default:
	}
}

func mustVerifier(t *testing.T) string {
	t.Helper()
	v, err := generateVerifier()
	if err != nil {
		t.Fatalf("generateVerifier: %v", err)
	}
	return v
}
