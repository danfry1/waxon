package auth

import (
	"errors"
	"fmt"
	"os"
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

func TestSaveTokenCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	path := filepath.Join(nested, "token.json")

	tok := &oauth2.Token{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-def",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	// Verify directory was created.
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}

	// Verify file exists.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("token file not created: %v", err)
	}
}

func TestLoadTokenInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	if err := os.WriteFile(path, []byte("not valid json!!!"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadToken(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSaveTokenPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	tok := &oauth2.Token{
		AccessToken:  "access-perm",
		RefreshToken: "refresh-perm",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

// staticTokenSource is a test helper that always returns the same token.
type staticTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s *staticTokenSource) Token() (*oauth2.Token, error) {
	return s.token, s.err
}

func TestPersistingTokenSource_NoRefresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	tok := &oauth2.Token{
		AccessToken:  "same-token",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	pts := NewPersistingTokenSource(&staticTokenSource{token: tok}, path, tok)

	got, err := pts.Token()
	if err != nil {
		t.Fatalf("Token(): %v", err)
	}
	if got.AccessToken != "same-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "same-token")
	}

	// File should NOT have been written since the token didn't change.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected token file not to be written when token is unchanged")
	}
}

func TestPersistingTokenSource_RefreshSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	original := &oauth2.Token{
		AccessToken:  "original-token",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	refreshed := &oauth2.Token{
		AccessToken:  "refreshed-token",
		RefreshToken: "refresh-2",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(2 * time.Hour),
	}

	// Base source returns the refreshed token (different AccessToken).
	pts := NewPersistingTokenSource(&staticTokenSource{token: refreshed}, path, original)

	got, err := pts.Token()
	if err != nil {
		t.Fatalf("Token(): %v", err)
	}
	if got.AccessToken != "refreshed-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "refreshed-token")
	}

	// File should have been written.
	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if loaded.AccessToken != "refreshed-token" {
		t.Errorf("saved AccessToken = %q, want %q", loaded.AccessToken, "refreshed-token")
	}
	if loaded.RefreshToken != "refresh-2" {
		t.Errorf("saved RefreshToken = %q, want %q", loaded.RefreshToken, "refresh-2")
	}
}

func TestPersistingTokenSource_BaseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	original := &oauth2.Token{
		AccessToken:  "orig",
		RefreshToken: "ref",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	baseErr := errors.New("token refresh failed")
	pts := NewPersistingTokenSource(&staticTokenSource{token: nil, err: baseErr}, path, original)

	_, err := pts.Token()
	if err == nil {
		t.Fatal("expected error when base source fails")
	}
	if err.Error() != "token refresh failed" {
		t.Errorf("error = %q, want %q", err.Error(), "token refresh failed")
	}
}

func TestSaveTokenRoundTripsAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	expiry := time.Date(2027, 1, 15, 12, 30, 0, 0, time.UTC)
	tok := &oauth2.Token{
		AccessToken:  "at-xyz",
		RefreshToken: "rt-abc",
		TokenType:    "Bearer",
		Expiry:       expiry,
	}

	if err := SaveToken(path, tok); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}

	if loaded.TokenType != tok.TokenType {
		t.Errorf("TokenType = %q, want %q", loaded.TokenType, tok.TokenType)
	}
	if !loaded.Expiry.Equal(tok.Expiry) {
		t.Errorf("Expiry = %v, want %v", loaded.Expiry, tok.Expiry)
	}
}

func TestDefaultTokenPathContainsSpotivim(t *testing.T) {
	path := DefaultTokenPath()
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

// ---------------------------------------------------------------------------
// SaveToken overwrites previous token
// ---------------------------------------------------------------------------

func TestSaveTokenOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	tok1 := &oauth2.Token{
		AccessToken:  "first-access",
		RefreshToken: "first-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	tok2 := &oauth2.Token{
		AccessToken:  "second-access",
		RefreshToken: "second-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Save first token
	if err := SaveToken(path, tok1); err != nil {
		t.Fatalf("SaveToken(first): %v", err)
	}

	// Save second token to same path
	if err := SaveToken(path, tok2); err != nil {
		t.Fatalf("SaveToken(second): %v", err)
	}

	// Load and verify the second token is returned
	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}

	if loaded.AccessToken != "second-access" {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, "second-access")
	}
	if loaded.RefreshToken != "second-refresh" {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, "second-refresh")
	}
	if !loaded.Expiry.Equal(tok2.Expiry) {
		t.Errorf("Expiry = %v, want %v", loaded.Expiry, tok2.Expiry)
	}
}

// ---------------------------------------------------------------------------
// PersistingTokenSource with invalid save path
// ---------------------------------------------------------------------------

func TestPersistingTokenSource_SaveError(t *testing.T) {
	// Use a path that can never be written to
	impossiblePath := "/dev/null/impossible/nested/token.json"

	original := &oauth2.Token{
		AccessToken:  "orig-token",
		RefreshToken: "orig-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	refreshed := &oauth2.Token{
		AccessToken:  "refreshed-token",
		RefreshToken: "refreshed-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(2 * time.Hour),
	}

	// Base source returns a different token (triggers save attempt)
	pts := NewPersistingTokenSource(&staticTokenSource{token: refreshed}, impossiblePath, original)

	// Token() should still succeed — save failure is logged, not returned
	got, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() should not fail even if save fails: %v", err)
	}
	if got.AccessToken != "refreshed-token" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "refreshed-token")
	}
}

// ---------------------------------------------------------------------------
// PersistingTokenSource — multiple refreshes update lastSaved
// ---------------------------------------------------------------------------

func TestPersistingTokenSource_MultipleRefreshes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	original := &oauth2.Token{
		AccessToken: "v1",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}

	// refreshingTokenSource returns a new token each call
	callNum := 0
	rts := &refreshingTokenSource{fn: func() (*oauth2.Token, error) {
		callNum++
		return &oauth2.Token{
			AccessToken:  fmt.Sprintf("v%d", callNum+1),
			RefreshToken: "refresh",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}, nil
	}}

	pts := NewPersistingTokenSource(rts, path, original)

	// First call: v1 -> v2 (save)
	tok1, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() call 1: %v", err)
	}
	if tok1.AccessToken != "v2" {
		t.Errorf("call 1: AccessToken = %q, want %q", tok1.AccessToken, "v2")
	}

	// Second call: v2 -> v3 (save again)
	tok2, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() call 2: %v", err)
	}
	if tok2.AccessToken != "v3" {
		t.Errorf("call 2: AccessToken = %q, want %q", tok2.AccessToken, "v3")
	}

	// Verify the last saved token on disk
	loaded, err := LoadToken(path)
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if loaded.AccessToken != "v3" {
		t.Errorf("saved AccessToken = %q, want %q", loaded.AccessToken, "v3")
	}
}

// refreshingTokenSource is a test helper that calls a function for each Token().
type refreshingTokenSource struct {
	fn func() (*oauth2.Token, error)
}

func (r *refreshingTokenSource) Token() (*oauth2.Token, error) {
	return r.fn()
}

// ---------------------------------------------------------------------------
// NewPersistingTokenSource fields
// ---------------------------------------------------------------------------

func TestNewPersistingTokenSourceFields(t *testing.T) {
	tok := &oauth2.Token{
		AccessToken: "test-access",
		TokenType:   "Bearer",
	}
	base := &staticTokenSource{token: tok}

	pts := NewPersistingTokenSource(base, "/some/path/token.json", tok)

	if pts == nil {
		t.Fatal("NewPersistingTokenSource returned nil")
	}
	if pts.path != "/some/path/token.json" {
		t.Errorf("path = %q, want %q", pts.path, "/some/path/token.json")
	}
	if pts.lastSaved != "test-access" {
		t.Errorf("lastSaved = %q, want %q", pts.lastSaved, "test-access")
	}
}
