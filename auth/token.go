package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

func DefaultTokenPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	return filepath.Join(configDir, "waxon", "token.json")
}

func SaveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	// Atomic write: write to temp file then rename to prevent corruption
	// if the process is interrupted mid-write.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read token: %w", err)
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &token, nil
}

// PersistingTokenSource wraps an oauth2.TokenSource and saves refreshed
// tokens to disk so they survive across app restarts.
type PersistingTokenSource struct {
	base      oauth2.TokenSource
	path      string
	mu        sync.Mutex
	lastSaved string
}

// NewPersistingTokenSource wraps base so that any refreshed token is
// automatically written to path.
func NewPersistingTokenSource(base oauth2.TokenSource, path string, current *oauth2.Token) *PersistingTokenSource {
	return &PersistingTokenSource{
		base:      base,
		path:      path,
		lastSaved: current.AccessToken,
	}
}

func (p *PersistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}

	if tok.AccessToken != p.lastSaved {
		// Token was refreshed — persist to disk.
		// Log but don't fail: the in-memory token still works for this session,
		// but the next startup may require re-auth if this keeps failing.
		if saveErr := SaveToken(p.path, tok); saveErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "warning: failed to save refreshed token: %v\n", saveErr)
		} else {
			p.lastSaved = tok.AccessToken
		}
	}

	return tok, nil
}
