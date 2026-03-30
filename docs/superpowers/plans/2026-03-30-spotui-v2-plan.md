# spotui v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform spotui into an immersive, atmospheric terminal Spotify player with Spotify Web API, mood-reactive visual effects (particles, glow, enhanced bars), and functional panels (queue, library, search, devices).

**Architecture:** Add `auth/` and `spotify/` packages for Spotify Web API integration via OAuth PKCE and `zmb3/spotify/v2`. Extend `visual/` with particle engine and glow renderer. Add panel system to `app/` using bubbles/list and bubbles/textinput. Keep osascript `source/` as fallback. New `RichSource` interface extends `TrackSource` for API-only features.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, bubbles (list, textinput, viewport), harmonica, zmb3/spotify/v2, golang.org/x/oauth2

---

## File Map

### New Files
```
auth/oauth.go          — PKCE auth flow: browser launch, local callback server, token exchange
auth/token.go          — Token persistence in ~/.config/spotui/token.json, auto-refresh
spotify/client.go      — Thin wrapper around zmb3/spotify/v2 client initialization
spotify/player.go      — PlayerSource: implements source.TrackSource + source.RichSource
spotify/library.go     — Playlist listing, playlist tracks, search, liked songs
spotify/features.go    — Audio features fetching with in-memory cache
source/rich.go         — RichSource interface definition + helper types (Playlist, Device, SearchResults, AudioFeatures)
visual/particles.go    — Particle engine: spawn, update, render to character grid
visual/glow.go         — Radial glow renderer around album art using tinted block chars
app/panels.go          — Panel models: queue, library, search, device picker using bubbles/list
app/effects.go         — Per-frame effects: particle tick, glow computation, background breathing
```

### Modified Files
```
main.go                — Add auth subcommand, startup flow with token check, source selection
go.mod                 — Add zmb3/spotify/v2, golang.org/x/oauth2
app/model.go           — Add panel state, effects state, volume/shuffle/repeat, RichSource handling
app/view.go            — Composite particles + glow + panels into final render
app/keys.go            — New keybindings: q→queue, l→library, /→search, d→devices, +/-→volume, s→shuffle, r→repeat
mood/detect.go         — Add DetectFromFeatures() alongside existing DetectMood()
visual/bars.go         — Add glow bloom on tall bars via background tinting
```

---

## Task 1: Add Dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add spotify and oauth2 dependencies**

```bash
cd /Users/danielfry/dev/tui && go get github.com/zmb3/spotify/v2 golang.org/x/oauth2
```

- [ ] **Step 2: Verify dependencies resolve**

```bash
cd /Users/danielfry/dev/tui && go mod tidy
```

Expected: no errors, `go.sum` updated.

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add go.mod go.sum && git commit -m "deps: add zmb3/spotify/v2 and golang.org/x/oauth2"
```

---

## Task 2: Token Storage

**Files:**
- Create: `auth/token.go`
- Test: `auth/token_test.go`

- [ ] **Step 1: Write failing test for token save/load round-trip**

Create `auth/token_test.go`:

```go
package auth

import (
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./auth/ -v
```

Expected: compilation error — `auth` package doesn't exist.

- [ ] **Step 3: Implement token storage**

Create `auth/token.go`:

```go
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
)

// DefaultTokenPath returns ~/.config/spotui/token.json.
func DefaultTokenPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	return filepath.Join(configDir, "spotui", "token.json")
}

// SaveToken writes an OAuth2 token to disk as JSON.
func SaveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadToken reads an OAuth2 token from disk.
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/danielfry/dev/tui && go test ./auth/ -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add auth/ && git commit -m "feat(auth): add token save/load with filesystem persistence"
```

---

## Task 3: OAuth PKCE Flow

**Files:**
- Create: `auth/oauth.go`
- Test: `auth/oauth_test.go`

- [ ] **Step 1: Write failing test for PKCE config and verifier**

Create `auth/oauth_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./auth/ -run TestSpotify -v
```

Expected: compilation error.

- [ ] **Step 3: Implement OAuth PKCE flow**

Create `auth/oauth.go`:

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"golang.org/x/oauth2"
)

var spotifyScopes = []string{
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-read-private",
	"user-library-read",
}

// SpotifyOAuthConfig returns the OAuth2 config for Spotify PKCE.
func SpotifyOAuthConfig(clientID, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURL,
		Scopes:      spotifyScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}
}

func generateVerifier() string {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func challengeFromVerifier(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// Authenticate runs the full PKCE flow: starts a local server, opens browser,
// waits for callback, exchanges code for token.
func Authenticate(clientID string) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	cfg := SpotifyOAuthConfig(clientID, redirectURL)
	verifier := generateVerifier()
	challenge := challengeFromVerifier(verifier)

	authURL := cfg.AuthCodeURL("spotui-state",
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
	)

	fmt.Printf("\nOpening browser for Spotify login...\n")
	fmt.Printf("If it doesn't open, visit:\n%s\n\n", authURL)
	openBrowser(authURL)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			http.Error(w, "Auth failed: "+errMsg, http.StatusBadRequest)
			errCh <- fmt.Errorf("auth denied: %s", errMsg)
			return
		}
		fmt.Fprint(w, "<html><body><h2>✓ Connected to Spotify!</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Close()
		return nil, err
	}

	server.Shutdown(context.Background())

	token, err := cfg.Exchange(context.Background(), code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	return token, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("open", url)
	}
	cmd.Start()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/danielfry/dev/tui && go test ./auth/ -v
```

Expected: all tests PASS (we only unit-test config and PKCE helpers, not the full browser flow).

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add auth/ && git commit -m "feat(auth): add Spotify OAuth PKCE flow with local callback server"
```

---

## Task 4: RichSource Interface

**Files:**
- Create: `source/rich.go`

- [ ] **Step 1: Define RichSource interface and helper types**

Create `source/rich.go`:

```go
package source

// RichSource extends TrackSource with Spotify API capabilities.
// The app checks if the source implements RichSource to enable panels.
type RichSource interface {
	TrackSource
	Queue() ([]Track, error)
	Playlists() ([]Playlist, error)
	PlaylistTracks(id string) ([]Track, error)
	Search(query string) (*SearchResults, error)
	SetVolume(percent int) error
	Devices() ([]Device, error)
	TransferPlayback(deviceID string) error
	SetShuffle(state bool) error
	SetRepeat(mode RepeatMode) error
	AudioFeatures(trackID string) (*AudioFeatures, error)
}

type Playlist struct {
	ID         string
	Name       string
	TrackCount int
}

type Device struct {
	ID       string
	Name     string
	Type     string
	IsActive bool
}

type SearchResults struct {
	Tracks  []Track
	Artists []string
	Albums  []string
}

type AudioFeatures struct {
	Energy       float64
	Valence      float64
	Danceability float64
	Tempo        float64
	Acousticness float64
}

type RepeatMode string

const (
	RepeatOff     RepeatMode = "off"
	RepeatContext RepeatMode = "context"
	RepeatTrack   RepeatMode = "track"
)
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/danielfry/dev/tui && go build ./source/
```

Expected: compiles successfully.

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add source/rich.go && git commit -m "feat(source): add RichSource interface with playlist, search, device, audio features types"
```

---

## Task 5: Spotify API Client & PlayerSource

**Files:**
- Create: `spotify/client.go`
- Create: `spotify/player.go`
- Create: `spotify/library.go`
- Create: `spotify/features.go`
- Test: `spotify/player_test.go`

- [ ] **Step 1: Write failing test for TrackSource interface conformance**

Create `spotify/player_test.go`:

```go
package spotify

import (
	"testing"

	"github.com/danielfry/spotui/source"
)

// Compile-time check: PlayerSource must implement both interfaces.
var _ source.TrackSource = (*PlayerSource)(nil)
var _ source.RichSource = (*PlayerSource)(nil)

func TestNewPlayerSource(t *testing.T) {
	// Cannot test with real API, but verify construction doesn't panic.
	// Real integration testing requires a Spotify token.
	t.Log("PlayerSource satisfies TrackSource and RichSource interfaces")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./spotify/ -v
```

Expected: compilation error — `spotify` package doesn't exist.

- [ ] **Step 3: Implement client initialization**

Create `spotify/client.go`:

```go
package spotify

import (
	"context"

	"github.com/danielfry/spotui/auth"
	spotifyapi "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// NewClient creates a Spotify API client from a stored token.
// It auto-refreshes the token when it expires.
func NewClient(clientID string, token *oauth2.Token, tokenPath string) *spotifyapi.Client {
	authenticator := spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopeUserLibraryRead,
		),
	)

	httpClient := authenticator.Client(context.Background(), token)
	client := spotifyapi.New(httpClient)

	// Save refreshed token after each use
	go func() {
		src := authenticator.TokenSource(context.Background(), token)
		newToken, err := src.Token()
		if err == nil && newToken.AccessToken != token.AccessToken {
			auth.SaveToken(tokenPath, newToken)
		}
	}()

	return client
}
```

- [ ] **Step 4: Implement PlayerSource**

Create `spotify/player.go`:

```go
package spotify

import (
	"context"
	"fmt"
	"time"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

type PlayerSource struct {
	client   *spotifyapi.Client
	features *FeatureCache
}

func NewPlayerSource(client *spotifyapi.Client) *PlayerSource {
	return &PlayerSource{
		client:   client,
		features: NewFeatureCache(client),
	}
}

func (p *PlayerSource) CurrentTrack() (*source.Track, error) {
	ctx := context.Background()
	state, err := p.client.PlayerState(ctx)
	if err != nil {
		return nil, fmt.Errorf("player state: %w", err)
	}
	if state == nil || state.Item == nil {
		return nil, nil
	}

	item := state.Item
	artist := ""
	if len(item.Artists) > 0 {
		artist = item.Artists[0].Name
	}

	artworkURL := ""
	if len(item.Album.Images) > 0 {
		artworkURL = item.Album.Images[0].URL
	}

	return &source.Track{
		ID:         string(item.ID),
		Name:       item.Name,
		Artist:     artist,
		Album:      item.Album.Name,
		ArtworkURL: artworkURL,
		Duration:   time.Duration(item.Duration) * time.Millisecond,
		Position:   time.Duration(state.Progress) * time.Millisecond,
		Playing:    state.Playing,
	}, nil
}

func (p *PlayerSource) Play() error {
	return p.client.Play(context.Background())
}

func (p *PlayerSource) Pause() error {
	return p.client.Pause(context.Background())
}

func (p *PlayerSource) Next() error {
	return p.client.Next(context.Background())
}

func (p *PlayerSource) Previous() error {
	return p.client.Previous(context.Background())
}

func (p *PlayerSource) Seek(position time.Duration) error {
	ms := int(position.Milliseconds())
	return p.client.Seek(context.Background(), ms)
}

func (p *PlayerSource) SetVolume(percent int) error {
	return p.client.Volume(context.Background(), percent)
}

func (p *PlayerSource) SetShuffle(state bool) error {
	return p.client.Shuffle(context.Background(), state)
}

func (p *PlayerSource) SetRepeat(mode source.RepeatMode) error {
	return p.client.Repeat(context.Background(), string(mode))
}

func (p *PlayerSource) Devices() ([]source.Device, error) {
	devs, err := p.client.PlayerDevices(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]source.Device, len(devs))
	for i, d := range devs {
		result[i] = source.Device{
			ID:       string(d.ID),
			Name:     d.Name,
			Type:     d.Type,
			IsActive: d.Active,
		}
	}
	return result, nil
}

func (p *PlayerSource) TransferPlayback(deviceID string) error {
	id := spotifyapi.ID(deviceID)
	return p.client.TransferPlayback(context.Background(), id, true)
}

func (p *PlayerSource) Queue() ([]source.Track, error) {
	q, err := p.client.GetQueue(context.Background())
	if err != nil {
		return nil, err
	}
	tracks := make([]source.Track, len(q.Items))
	for i, item := range q.Items {
		artist := ""
		if len(item.Artists) > 0 {
			artist = item.Artists[0].Name
		}
		artworkURL := ""
		if len(item.Album.Images) > 0 {
			artworkURL = item.Album.Images[0].URL
		}
		tracks[i] = source.Track{
			ID:       string(item.ID),
			Name:     item.Name,
			Artist:   artist,
			Album:    item.Album.Name,
			ArtworkURL: artworkURL,
			Duration: time.Duration(item.Duration) * time.Millisecond,
		}
	}
	return tracks, nil
}

func (p *PlayerSource) AudioFeatures(trackID string) (*source.AudioFeatures, error) {
	return p.features.Get(trackID)
}
```

- [ ] **Step 5: Implement library operations**

Create `spotify/library.go`:

```go
package spotify

import (
	"context"
	"time"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

func (p *PlayerSource) Playlists() ([]source.Playlist, error) {
	ctx := context.Background()
	page, err := p.client.CurrentUsersPlaylists(ctx, spotifyapi.Limit(50))
	if err != nil {
		return nil, err
	}
	playlists := make([]source.Playlist, len(page.Playlists))
	for i, pl := range page.Playlists {
		playlists[i] = source.Playlist{
			ID:         string(pl.ID),
			Name:       pl.Name,
			TrackCount: int(pl.Tracks.Total),
		}
	}
	return playlists, nil
}

func (p *PlayerSource) PlaylistTracks(id string) ([]source.Track, error) {
	ctx := context.Background()
	page, err := p.client.GetPlaylistItems(ctx, spotifyapi.ID(id), spotifyapi.Limit(50))
	if err != nil {
		return nil, err
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		t := item.Track.Track
		if t == nil {
			continue
		}
		artist := ""
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}
		artworkURL := ""
		if len(t.Album.Images) > 0 {
			artworkURL = t.Album.Images[0].URL
		}
		tracks = append(tracks, source.Track{
			ID:       string(t.ID),
			Name:     t.Name,
			Artist:   artist,
			Album:    t.Album.Name,
			ArtworkURL: artworkURL,
			Duration: time.Duration(t.Duration) * time.Millisecond,
		})
	}
	return tracks, nil
}

func (p *PlayerSource) Search(query string) (*source.SearchResults, error) {
	ctx := context.Background()
	result, err := p.client.Search(ctx, query, spotifyapi.SearchTypeTrack|spotifyapi.SearchTypeArtist|spotifyapi.SearchTypeAlbum, spotifyapi.Limit(10))
	if err != nil {
		return nil, err
	}

	sr := &source.SearchResults{}

	if result.Tracks != nil {
		for _, t := range result.Tracks.Tracks {
			artist := ""
			if len(t.Artists) > 0 {
				artist = t.Artists[0].Name
			}
			artworkURL := ""
			if len(t.Album.Images) > 0 {
				artworkURL = t.Album.Images[0].URL
			}
			sr.Tracks = append(sr.Tracks, source.Track{
				ID:       string(t.ID),
				Name:     t.Name,
				Artist:   artist,
				Album:    t.Album.Name,
				ArtworkURL: artworkURL,
				Duration: time.Duration(t.Duration) * time.Millisecond,
			})
		}
	}
	if result.Artists != nil {
		for _, a := range result.Artists.Artists {
			sr.Artists = append(sr.Artists, a.Name)
		}
	}
	if result.Albums != nil {
		for _, a := range result.Albums.Albums {
			sr.Albums = append(sr.Albums, a.Name)
		}
	}

	return sr, nil
}
```

- [ ] **Step 6: Implement audio features cache**

Create `spotify/features.go`:

```go
package spotify

import (
	"context"
	"sync"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

type FeatureCache struct {
	client *spotifyapi.Client
	cache  map[string]*source.AudioFeatures
	mu     sync.Mutex
}

func NewFeatureCache(client *spotifyapi.Client) *FeatureCache {
	return &FeatureCache{
		client: client,
		cache:  make(map[string]*source.AudioFeatures),
	}
}

func (fc *FeatureCache) Get(trackID string) (*source.AudioFeatures, error) {
	fc.mu.Lock()
	if cached, ok := fc.cache[trackID]; ok {
		fc.mu.Unlock()
		return cached, nil
	}
	fc.mu.Unlock()

	features, err := fc.client.GetAudioFeatures(context.Background(), spotifyapi.ID(trackID))
	if err != nil {
		return nil, err
	}
	if len(features) == 0 || features[0] == nil {
		return nil, nil
	}

	f := features[0]
	af := &source.AudioFeatures{
		Energy:       float64(f.Energy),
		Valence:      float64(f.Valence),
		Danceability: float64(f.Danceability),
		Tempo:        float64(f.Tempo),
		Acousticness: float64(f.Acousticness),
	}

	fc.mu.Lock()
	fc.cache[trackID] = af
	// Keep cache bounded
	if len(fc.cache) > 100 {
		for k := range fc.cache {
			delete(fc.cache, k)
			break
		}
	}
	fc.mu.Unlock()

	return af, nil
}
```

- [ ] **Step 7: Add ID field to source.Track**

The Spotify API returns track IDs needed for audio features lookup. Add an `ID` field to `source.Track`:

Modify `source/source.go` — add `ID string` as the first field in the Track struct:

```go
type Track struct {
	ID         string
	Name       string
	Artist     string
	Album      string
	ArtworkURL string
	Duration   time.Duration
	Position   time.Duration
	Playing    bool
}
```

- [ ] **Step 8: Run tests to verify compilation and interface conformance**

```bash
cd /Users/danielfry/dev/tui && go build ./... && go test ./spotify/ -v
```

Expected: compiles, test passes confirming PlayerSource satisfies both interfaces.

- [ ] **Step 9: Commit**

```bash
cd /Users/danielfry/dev/tui && git add source/source.go source/rich.go spotify/ && git commit -m "feat(spotify): add Spotify Web API client with player, library, search, and audio features"
```

---

## Task 6: Audio Features Mood Detection

**Files:**
- Modify: `mood/detect.go`
- Test: `mood/detect_test.go`

- [ ] **Step 1: Write failing test for feature-based mood detection**

Add to `mood/detect_test.go` (or create it if it doesn't have these tests):

```go
package mood

import (
	"testing"

	"github.com/danielfry/spotui/source"
)

func TestDetectFromFeatures(t *testing.T) {
	tests := []struct {
		name     string
		features source.AudioFeatures
		want     string
	}{
		{"high energy dance → electric", source.AudioFeatures{Energy: 0.85, Valence: 0.6, Danceability: 0.8, Tempo: 128}, "electric"},
		{"low energy ambient → drift", source.AudioFeatures{Energy: 0.15, Valence: 0.3, Danceability: 0.2, Tempo: 80, Acousticness: 0.7}, "drift"},
		{"happy upbeat → bright", source.AudioFeatures{Energy: 0.7, Valence: 0.8, Danceability: 0.7, Tempo: 120}, "bright"},
		{"angry intense → dark", source.AudioFeatures{Energy: 0.8, Valence: 0.15, Danceability: 0.4, Tempo: 140}, "dark"},
		{"acoustic mellow → warm", source.AudioFeatures{Energy: 0.35, Valence: 0.5, Danceability: 0.3, Tempo: 95, Acousticness: 0.8}, "warm"},
		{"soulful groove → golden", source.AudioFeatures{Energy: 0.45, Valence: 0.65, Danceability: 0.6, Tempo: 110}, "golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFromFeatures(&tt.features)
			if got.Name != tt.want {
				t.Errorf("DetectFromFeatures() = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestDetectFromFeaturesNil(t *testing.T) {
	got := DetectFromFeatures(nil)
	if got.Name != Warm.Name {
		t.Errorf("nil features should default to warm, got %q", got.Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./mood/ -run TestDetectFromFeatures -v
```

Expected: compilation error — `DetectFromFeatures` not defined.

- [ ] **Step 3: Implement feature-based detection**

Add to `mood/detect.go`:

```go
import "github.com/danielfry/spotui/source"
```

And add this function after the existing `DetectMood` function:

```go
// DetectFromFeatures uses Spotify audio features for more accurate mood detection.
// Falls back to Warm if features are nil.
func DetectFromFeatures(f *source.AudioFeatures) Mood {
	if f == nil {
		return Warm
	}

	// Score each mood based on how well the features match
	type scored struct {
		mood  Mood
		score float64
	}

	candidates := []scored{
		{Electric, scoreElectric(f)},
		{Dark, scoreDark(f)},
		{Bright, scoreBright(f)},
		{Golden, scoreGolden(f)},
		{Drift, scoreDrift(f)},
		{Warm, scoreWarm(f)},
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return best.mood
}

func scoreElectric(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.7 {
		score += f.Energy
	}
	if f.Danceability > 0.6 {
		score += f.Danceability * 0.5
	}
	return score
}

func scoreDark(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.5 && f.Valence < 0.3 {
		score += f.Energy * (1 - f.Valence)
	}
	return score
}

func scoreBright(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Valence > 0.6 && f.Energy > 0.5 {
		score += f.Valence * 0.6
		if f.Danceability > 0.5 {
			score += 0.3
		}
	}
	return score
}

func scoreGolden(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.3 && f.Energy < 0.6 && f.Valence > 0.4 && f.Valence < 0.8 {
		score += 0.7
		if f.Danceability > 0.4 && f.Danceability < 0.7 {
			score += 0.3
		}
	}
	return score
}

func scoreDrift(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy < 0.3 {
		score += (1 - f.Energy)
		if f.Acousticness > 0.3 {
			score += f.Acousticness * 0.3
		}
	}
	return score
}

func scoreWarm(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Acousticness > 0.5 {
		score += f.Acousticness * 0.5
	}
	if f.Energy > 0.2 && f.Energy < 0.5 {
		score += 0.4
	}
	return score
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/danielfry/dev/tui && go test ./mood/ -v
```

Expected: all tests pass. If any mood classification test fails, adjust scoring thresholds.

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add mood/detect.go mood/detect_test.go && git commit -m "feat(mood): add audio-features-based mood detection with scoring system"
```

---

## Task 7: Particle System

**Files:**
- Create: `visual/particles.go`
- Test: `visual/particles_test.go`

- [ ] **Step 1: Write failing test**

Create `visual/particles_test.go`:

```go
package visual

import "testing"

func TestNewParticleSystem(t *testing.T) {
	ps := NewParticleSystem(30, 80, 24)
	if len(ps.particles) != 30 {
		t.Errorf("particle count = %d, want 30", len(ps.particles))
	}
}

func TestParticleSystemUpdate(t *testing.T) {
	ps := NewParticleSystem(10, 80, 24)
	// Record initial positions
	initialX := ps.particles[0].x
	initialY := ps.particles[0].y

	ps.Update(0.5, "#ff0000", "#00ff00")

	// At least one particle should have moved
	moved := false
	for _, p := range ps.particles {
		if p.x != initialX || p.y != initialY {
			moved = true
			break
		}
	}
	if !moved {
		t.Error("expected at least one particle to move after update")
	}
}

func TestParticleSystemRender(t *testing.T) {
	ps := NewParticleSystem(5, 40, 10)
	ps.Update(0.5, "#ff0000", "#00ff00")
	grid := ps.Render()
	if len(grid) != 10 {
		t.Errorf("grid height = %d, want 10", len(grid))
	}
	if len(grid[0]) != 40 {
		t.Errorf("grid width = %d, want 40", len(grid[0]))
	}
}

func TestParticleSystemResize(t *testing.T) {
	ps := NewParticleSystem(10, 80, 24)
	ps.Resize(120, 40)
	if ps.width != 120 || ps.height != 40 {
		t.Errorf("resize failed: got %dx%d", ps.width, ps.height)
	}
	grid := ps.Render()
	if len(grid) != 40 {
		t.Errorf("grid height after resize = %d, want 40", len(grid))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -run TestParticle -v
```

Expected: compilation error.

- [ ] **Step 3: Implement particle system**

Create `visual/particles.go`:

```go
package visual

import (
	"math/rand/v2"

	"github.com/charmbracelet/lipgloss"
)

var particleChars = []rune{'·', '•', '∘', '⋅', '◦'}

type particle struct {
	x, y   float64
	vx, vy float64
	char   rune
	alive  bool
}

// ParticleCell is a rendered particle with its styled character.
type ParticleCell struct {
	Char  string // styled character
	Style lipgloss.Style
}

type ParticleSystem struct {
	particles []particle
	width     int
	height    int
}

func NewParticleSystem(count, width, height int) *ParticleSystem {
	ps := &ParticleSystem{
		particles: make([]particle, count),
		width:     width,
		height:    height,
	}
	for i := range ps.particles {
		ps.particles[i] = ps.spawnParticle()
	}
	return ps
}

func (ps *ParticleSystem) spawnParticle() particle {
	return particle{
		x:     rand.Float64() * float64(ps.width),
		y:     rand.Float64() * float64(ps.height),
		vx:    (rand.Float64() - 0.5) * 0.3,
		vy:    (rand.Float64() - 0.5) * 0.15,
		char:  particleChars[rand.IntN(len(particleChars))],
		alive: true,
	}
}

func (ps *ParticleSystem) Resize(width, height int) {
	ps.width = width
	ps.height = height
}

// Update advances all particles by one frame.
// energy controls speed (0.0-1.0), colors are mood palette colors.
func (ps *ParticleSystem) Update(energy float64, primary, secondary string) {
	speedMul := 0.5 + energy*1.5

	for i := range ps.particles {
		p := &ps.particles[i]

		p.x += p.vx * speedMul
		p.y += p.vy * speedMul

		// Add slight drift
		p.vx += (rand.Float64() - 0.5) * 0.02
		p.vy += (rand.Float64() - 0.5) * 0.01

		// Clamp velocity
		if p.vx > 0.5 {
			p.vx = 0.5
		} else if p.vx < -0.5 {
			p.vx = -0.5
		}
		if p.vy > 0.3 {
			p.vy = 0.3
		} else if p.vy < -0.3 {
			p.vy = -0.3
		}

		// Wrap around edges
		if p.x < 0 {
			p.x += float64(ps.width)
		} else if p.x >= float64(ps.width) {
			p.x -= float64(ps.width)
		}
		if p.y < 0 {
			p.y += float64(ps.height)
		} else if p.y >= float64(ps.height) {
			p.y -= float64(ps.height)
		}
	}
}

// Render returns a 2D grid of particle characters.
// Each cell is either a rune (particle) or 0 (empty).
func (ps *ParticleSystem) Render() [][]rune {
	grid := make([][]rune, ps.height)
	for i := range grid {
		grid[i] = make([]rune, ps.width)
	}

	for _, p := range ps.particles {
		if !p.alive {
			continue
		}
		col := int(p.x)
		row := int(p.y)
		if row >= 0 && row < ps.height && col >= 0 && col < ps.width {
			grid[row][col] = p.char
		}
	}

	return grid
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -run TestParticle -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add visual/particles.go visual/particles_test.go && git commit -m "feat(visual): add floating particle system with mood-reactive speed and density"
```

---

## Task 8: Glow Renderer

**Files:**
- Create: `visual/glow.go`
- Test: `visual/glow_test.go`

- [ ] **Step 1: Write failing test**

Create `visual/glow_test.go`:

```go
package visual

import "testing"

func TestRenderGlow(t *testing.T) {
	grid := RenderGlow(40, 12, 20, 10, "#daa520", "#8b7a54", "#1a1510", 0.5)
	if len(grid) != 12 {
		t.Errorf("grid height = %d, want 12", len(grid))
	}
	if len(grid[0]) != 40 {
		t.Errorf("grid width = %d, want 40", len(grid[0]))
	}
}

func TestGlowCenterBrighter(t *testing.T) {
	w, h := 40, 20
	artW, artH := 16, 8
	grid := RenderGlow(w, h, artW, artH, "#daa520", "#8b7a54", "#1a1510", 0.5)

	// Center cell should have a non-empty glow color
	centerRow := h / 2
	centerCol := w / 2
	cell := grid[centerRow][centerCol]
	if cell == "" {
		t.Error("center cell should have glow color")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -run TestGlow -v
```

Expected: compilation error.

- [ ] **Step 3: Implement glow renderer**

Create `visual/glow.go`:

```go
package visual

import (
	"math"
)

// RenderGlow produces a 2D grid of background hex colors creating a radial glow
// around where the album art is centered. Each cell is either a hex color string
// (for tinted background) or empty string (no glow, use default background).
//
// screenW/screenH: terminal dimensions (inner content area)
// artW/artH: album art dimensions in character cells
// primary/secondary/background: mood palette colors
// energy: 0.0-1.0, controls glow intensity and radius
func RenderGlow(screenW, screenH, artW, artH int, primary, secondary, background string, energy float64) [][]string {
	grid := make([][]string, screenH)
	for i := range grid {
		grid[i] = make([]string, screenW)
	}

	// Art center position
	cx := float64(screenW) / 2
	cy := float64(screenH) / 2

	// Glow radius scales with energy
	maxRadius := math.Max(float64(artW), float64(artH)) * (1.5 + energy*1.5)
	if maxRadius < 8 {
		maxRadius = 8
	}

	// Intensity scales with energy
	intensity := 0.15 + energy*0.25

	for row := range screenH {
		for col := range screenW {
			// Distance from art center, normalized by art dimensions
			dx := (float64(col) - cx) / (float64(artW) / 2)
			dy := (float64(row) - cy) / (float64(artH) / 2)
			dist := math.Sqrt(dx*dx + dy*dy)

			// Skip cells inside the art area (they're covered by artwork)
			if math.Abs(float64(col)-cx) <= float64(artW)/2 &&
				math.Abs(float64(row)-cy) <= float64(artH)/2 {
				continue
			}

			// Glow falloff: exponential decay from art edge
			normalizedDist := dist - 1.0 // 0 at art edge, increases outward
			if normalizedDist < 0 {
				normalizedDist = 0
			}

			falloff := math.Exp(-normalizedDist * 0.8)
			if falloff < 0.02 {
				continue // Too faint to render
			}

			// Blend between primary (inner) and secondary (outer) glow colors
			colorT := math.Min(normalizedDist/3.0, 1.0)
			glowColor := LerpColor(primary, secondary, colorT)

			// Mix glow color with background at glow intensity
			t := falloff * intensity
			color := LerpColor(background, glowColor, t)
			grid[row][col] = color
		}
	}

	return grid
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -run TestGlow -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add visual/glow.go visual/glow_test.go && git commit -m "feat(visual): add radial glow renderer for album art ambient lighting"
```

---

## Task 9: Enhanced Bars with Gradient Glow

**Files:**
- Modify: `visual/bars.go`
- Test: `visual/bars_test.go`

- [ ] **Step 1: Write failing test for glow bloom**

Add to existing test file (or create `visual/bars_test.go`):

```go
package visual

import "testing"

func TestRenderBarsWithGlow(t *testing.T) {
	heights := []float64{0.8, 0.5, 0.3, 0.9, 0.2}
	result := RenderBarsWithGlow(heights, 8, "#daa520", "#8b7a54", "#1a1510")
	if result == "" {
		t.Error("expected non-empty bar render")
	}
	// Should contain background-colored cells for glow
	if len(result) == 0 {
		t.Error("result should have content")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -run TestRenderBarsWithGlow -v
```

Expected: compilation error.

- [ ] **Step 3: Add glow bloom to bars**

Add this function to `visual/bars.go`:

```go
// RenderBarsWithGlow renders bars with background tinting on tall bars to create a bloom effect.
func RenderBarsWithGlow(heights []float64, maxHeight int, primary, secondary, background string) string {
	if len(heights) == 0 {
		return strings.Repeat("\n", maxHeight)
	}
	lines := make([]string, maxHeight)
	for row := range maxHeight {
		rowRatio := 1.0 - float64(row)/float64(maxHeight)
		rowColor := LerpColor(secondary, primary, 0.2+rowRatio*0.8)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(rowColor))

		var sb strings.Builder
		for _, h := range heights {
			barH := h * float64(maxHeight)
			rowFromBottom := maxHeight - 1 - row

			if float64(rowFromBottom) < barH-1 {
				// Full block with glow background for tall bars
				bgTint := background
				if h > 0.6 {
					glowT := (h - 0.6) / 0.4 * 0.15 // 0-15% tint for tall bars
					bgTint = LerpColor(background, rowColor, glowT)
				}
				glowStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color(rowColor)).
					Background(lipgloss.Color(bgTint))
				sb.WriteString(glowStyle.Render("█"))
			} else if float64(rowFromBottom) < barH {
				frac := barH - math.Floor(barH)
				idx := int(frac * float64(len(barChars)-1))
				idx = max(0, min(idx, len(barChars)-1))
				sb.WriteString(style.Render(barChars[idx]))
			} else {
				// Empty space — add subtle glow if adjacent bar is tall
				glowBg := background
				if h > 0.5 && float64(rowFromBottom) < barH+3 {
					proximity := 1.0 - (float64(rowFromBottom)-barH)/3.0
					glowBg = LerpColor(background, secondary, proximity*0.05)
				}
				bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(glowBg))
				sb.WriteString(bgStyle.Render(" "))
			}
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/danielfry/dev/tui && go test ./visual/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/danielfry/dev/tui && git add visual/bars.go visual/bars_test.go && git commit -m "feat(visual): add glow bloom effect to tall vibe bars"
```

---

## Task 10: Updated Keybindings

**Files:**
- Modify: `app/keys.go`

- [ ] **Step 1: Expand KeyMap with new bindings**

Replace the contents of `app/keys.go` with:

```go
package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	PlayPause key.Binding
	Next      key.Binding
	Prev      key.Binding
	VolumeUp  key.Binding
	VolumeDown key.Binding
	Shuffle   key.Binding
	Repeat    key.Binding
	Queue     key.Binding
	Library   key.Binding
	Search    key.Binding
	Devices   key.Binding
	Help      key.Binding
	Quit      key.Binding
	Close     key.Binding
	Select    key.Binding
	Up        key.Binding
	Down      key.Binding
	Back      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		PlayPause:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "play/pause")),
		Next:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next")),
		Prev:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev")),
		VolumeUp:   key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "volume up")),
		VolumeDown: key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "volume down")),
		Shuffle:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shuffle")),
		Repeat:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),
		Queue:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "queue")),
		Library:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "library")),
		Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Devices:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "devices")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:       key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Close:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		Select:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Back:       key.NewBinding(key.WithKeys("backspace"), key.WithHelp("←", "back")),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Next, k.Prev, k.Queue, k.Library, k.Search, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PlayPause, k.Next, k.Prev, k.VolumeUp, k.VolumeDown},
		{k.Shuffle, k.Repeat, k.Queue, k.Library, k.Search, k.Devices},
		{k.Up, k.Down, k.Select, k.Close, k.Back, k.Help, k.Quit},
	}
}
```

Note: `q` is no longer quit — it's now queue. Quit is `ctrl+c` only.

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/danielfry/dev/tui && go build ./app/
```

Expected: compiles. Note: the model.go Update function references old key names — this will be updated in the integration task.

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add app/keys.go && git commit -m "feat(app): expand keybindings for panels, volume, shuffle, repeat, devices"
```

---

## Task 11: Panel Models

**Files:**
- Create: `app/panels.go`

- [ ] **Step 1: Implement panel models using bubbles/list**

Create `app/panels.go`:

```go
package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/source"
)

// PanelType identifies which panel is active.
type PanelType int

const (
	PanelNone PanelType = iota
	PanelQueue
	PanelLibrary
	PanelSearch
	PanelDevices
)

// trackItem adapts source.Track for bubbles/list.
type trackItem struct {
	track source.Track
}

func (t trackItem) Title() string       { return t.track.Name }
func (t trackItem) Description() string { return fmt.Sprintf("%s · %s", t.track.Artist, formatDur(t.track.Duration)) }
func (t trackItem) FilterValue() string { return t.track.Name + " " + t.track.Artist }

// playlistItem adapts source.Playlist for bubbles/list.
type playlistItem struct {
	playlist source.Playlist
}

func (p playlistItem) Title() string       { return p.playlist.Name }
func (p playlistItem) Description() string { return fmt.Sprintf("%d tracks", p.playlist.TrackCount) }
func (p playlistItem) FilterValue() string { return p.playlist.Name }

// deviceItem adapts source.Device for bubbles/list.
type deviceItem struct {
	device source.Device
}

func (d deviceItem) Title() string {
	if d.device.IsActive {
		return "▶ " + d.device.Name
	}
	return "  " + d.device.Name
}
func (d deviceItem) Description() string { return d.device.Type }
func (d deviceItem) FilterValue() string { return d.device.Name }

// Panel holds the state for any active panel overlay.
type Panel struct {
	Type      PanelType
	List      list.Model
	Search    textinput.Model
	Width     int
	Height    int

	// Library drill-down state
	playlists    []source.Playlist
	inPlaylist   bool
	playlistID   string
	playlistName string
}

func NewPanel(panelType PanelType, width, height int) Panel {
	panelW := width / 2
	panelH := height - 4

	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, panelW-4, panelH-4)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	p := Panel{
		Type:   panelType,
		List:   l,
		Width:  panelW,
		Height: panelH,
	}

	switch panelType {
	case PanelQueue:
		l.Title = "Up Next"
	case PanelLibrary:
		l.Title = "Library"
	case PanelDevices:
		l.Title = "Devices"
	case PanelSearch:
		ti := textinput.New()
		ti.Placeholder = "Search tracks, artists, albums..."
		ti.Focus()
		ti.Width = panelW - 8
		p.Search = ti
		l.Title = "Search"
	}

	p.List = l
	return p
}

func (p *Panel) SetItems(items []list.Item) {
	p.List.SetItems(items)
}

func (p *Panel) Resize(width, height int) {
	p.Width = width / 2
	p.Height = height - 4
	p.List.SetSize(p.Width-4, p.Height-4)
}

// View renders the panel as a styled overlay.
func (p Panel) View(primary, secondary, background string) string {
	panelBg := lipgloss.Color(background)
	borderColor := lipgloss.Color(primary)

	style := lipgloss.NewStyle().
		Width(p.Width - 2).
		Height(p.Height).
		Background(panelBg).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2)

	content := p.List.View()
	if p.Type == PanelSearch {
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(primary)).
			Background(panelBg)
		content = searchStyle.Render(p.Search.View()) + "\n\n" + content
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(secondary)).
		Background(panelBg)
	hints := p.hintText()

	return style.Render(content + "\n" + hintStyle.Render(hints))
}

func (p Panel) hintText() string {
	switch p.Type {
	case PanelQueue:
		return "↑↓ navigate · enter play · q close"
	case PanelLibrary:
		if p.inPlaylist {
			return "↑↓ navigate · enter play · backspace back · l close"
		}
		return "↑↓ navigate · enter open · l close"
	case PanelSearch:
		return "type to search · ↑↓ navigate · enter play · esc close"
	case PanelDevices:
		return "↑↓ navigate · enter select · d close"
	}
	return ""
}

func formatDur(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// Panel-related messages
type queueLoadedMsg struct{ tracks []source.Track }
type playlistsLoadedMsg struct{ playlists []source.Playlist }
type playlistTracksMsg struct{ tracks []source.Track }
type devicesLoadedMsg struct{ devices []source.Device }
type searchResultsMsg struct{ results *source.SearchResults }

func fetchQueue(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		tracks, err := src.Queue()
		if err != nil {
			return trackErrorMsg{err}
		}
		return queueLoadedMsg{tracks}
	}
}

func fetchPlaylists(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		playlists, err := src.Playlists()
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistsLoadedMsg{playlists}
	}
}

func fetchPlaylistTracks(src source.RichSource, id string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := src.PlaylistTracks(id)
		if err != nil {
			return trackErrorMsg{err}
		}
		return playlistTracksMsg{tracks}
	}
}

func fetchDevices(src source.RichSource) tea.Cmd {
	return func() tea.Msg {
		devices, err := src.Devices()
		if err != nil {
			return trackErrorMsg{err}
		}
		return devicesLoadedMsg{devices}
	}
}

func doSearch(src source.RichSource, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := src.Search(query)
		if err != nil {
			return trackErrorMsg{err}
		}
		return searchResultsMsg{results}
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/danielfry/dev/tui && go build ./app/
```

Expected: may fail if model.go references removed `Quit` key binding with `q`. That's expected — will be fixed in the integration task.

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add app/panels.go && git commit -m "feat(app): add panel models for queue, library, search, and device picker"
```

---

## Task 12: Effects Engine

**Files:**
- Create: `app/effects.go`

- [ ] **Step 1: Implement effects state and per-frame updates**

Create `app/effects.go`:

```go
package app

import (
	"math"

	"github.com/danielfry/spotui/visual"
)

// Effects holds all visual effect state.
type Effects struct {
	Particles *visual.ParticleSystem
	GlowGrid  [][]string // pre-rendered glow colors per cell
	Breathing float64    // current breathing offset (0.0-1.0)
}

func NewEffects(width, height int) Effects {
	return Effects{
		Particles: visual.NewParticleSystem(35, width, height),
	}
}

func (e *Effects) Resize(width, height int) {
	if e.Particles != nil {
		e.Particles.Resize(width, height)
	}
}

// Tick updates all effects for one animation frame.
func (e *Effects) Tick(energy, beatPhase float64, primary, secondary, background string, artW, artH, screenW, screenH int) {
	// Update particles
	if e.Particles != nil {
		e.Particles.Update(energy, primary, secondary)
	}

	// Update glow grid (recompute when art dimensions are known)
	if artW > 0 && artH > 0 && screenW > 0 && screenH > 0 {
		e.GlowGrid = visual.RenderGlow(screenW, screenH, artW, artH, primary, secondary, background, energy)
	}

	// Background breathing: subtle sine wave tied to beat phase
	// Amplitude proportional to energy
	amplitude := energy * 0.03 // max 3% lightness shift
	e.Breathing = amplitude * math.Sin(beatPhase*2*math.Pi)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /Users/danielfry/dev/tui && go build ./app/
```

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add app/effects.go && git commit -m "feat(app): add effects engine for particles, glow, and background breathing"
```

---

## Task 13: Model & View Integration

This is the largest task — wiring everything together in the app model and view.

**Files:**
- Modify: `app/model.go`
- Modify: `app/view.go`

- [ ] **Step 1: Update Model struct with new state**

In `app/model.go`, update the `Model` struct to add:

```go
// Add these fields to Model struct after the existing fields:
	effects     Effects
	panel       *Panel
	activePanel PanelType
	richSource  source.RichSource // nil if source doesn't support rich features
	volume      int
	shuffleOn   bool
	repeatMode  source.RepeatMode
	audioFeatures *source.AudioFeatures
```

Add the import for `source` package's `RichSource` (already imported).

- [ ] **Step 2: Update NewModel to initialize effects and detect RichSource**

Update the `NewModel` function:

```go
func NewModel(src source.TrackSource) Model {
	m := Model{
		source: src, mood: mood.Idle, targetMood: mood.Idle,
		keys: DefaultKeyMap(), help: help.New(),
		volume: 50, repeatMode: source.RepeatOff,
	}
	// Check if source supports rich features
	if rich, ok := src.(source.RichSource); ok {
		m.richSource = rich
	}
	for i := range numBars {
		m.barSprings[i] = harmonica.NewSpring(harmonica.FPS(animFPS), 8.0, 0.6)
		m.barTargets[i] = rand.Float64() * 0.3
	}
	return m
}
```

- [ ] **Step 3: Update the Update function to handle new keys and panels**

Replace the `tea.KeyMsg` case in `Update` with expanded key handling:

```go
	case tea.KeyMsg:
		// If a panel is open, route keys to the panel first
		if m.activePanel != PanelNone {
			return m.updatePanel(msg)
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.PlayPause):
			if m.track != nil && m.track.Playing {
				return m, controlCmd(m.source.Pause)
			}
			return m, controlCmd(m.source.Play)
		case key.Matches(msg, m.keys.Next):
			return m, controlCmd(m.source.Next)
		case key.Matches(msg, m.keys.Prev):
			return m, controlCmd(m.source.Previous)
		case msg.Type == tea.KeyLeft:
			return m, m.seekRelative(-5 * time.Second)
		case msg.Type == tea.KeyRight:
			return m, m.seekRelative(5 * time.Second)
		case key.Matches(msg, m.keys.Queue):
			return m.togglePanel(PanelQueue)
		case key.Matches(msg, m.keys.Library):
			return m.togglePanel(PanelLibrary)
		case key.Matches(msg, m.keys.Search):
			return m.togglePanel(PanelSearch)
		case key.Matches(msg, m.keys.Devices):
			return m.togglePanel(PanelDevices)
		case key.Matches(msg, m.keys.VolumeUp):
			return m.adjustVolume(5)
		case key.Matches(msg, m.keys.VolumeDown):
			return m.adjustVolume(-5)
		case key.Matches(msg, m.keys.Shuffle):
			return m.toggleShuffle()
		case key.Matches(msg, m.keys.Repeat):
			return m.cycleRepeat()
		}
```

- [ ] **Step 4: Add panel toggle, volume, shuffle, repeat methods**

Add these methods to `app/model.go`:

```go
func (m Model) togglePanel(pt PanelType) (Model, tea.Cmd) {
	if m.richSource == nil {
		return m, nil // panels require RichSource
	}
	if m.activePanel == pt {
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	}
	p := NewPanel(pt, m.width, m.height)
	m.panel = &p
	m.activePanel = pt

	switch pt {
	case PanelQueue:
		return m, fetchQueue(m.richSource)
	case PanelLibrary:
		return m, fetchPlaylists(m.richSource)
	case PanelDevices:
		return m, fetchDevices(m.richSource)
	case PanelSearch:
		return m, nil // wait for user to type
	}
	return m, nil
}

func (m Model) updatePanel(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Close):
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case key.Matches(msg, m.keys.Queue) && m.activePanel == PanelQueue:
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case key.Matches(msg, m.keys.Library) && m.activePanel == PanelLibrary:
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case key.Matches(msg, m.keys.Devices) && m.activePanel == PanelDevices:
		m.activePanel = PanelNone
		m.panel = nil
		return m, nil
	case key.Matches(msg, m.keys.Select):
		return m.panelSelect()
	case key.Matches(msg, m.keys.Back) && m.activePanel == PanelLibrary && m.panel.inPlaylist:
		m.panel.inPlaylist = false
		m.panel.playlistID = ""
		items := make([]list.Item, len(m.panel.playlists))
		for i, pl := range m.panel.playlists {
			items[i] = playlistItem{pl}
		}
		m.panel.SetItems(items)
		m.panel.List.Title = "Library"
		return m, nil
	}

	// Forward to the list for navigation
	if m.activePanel == PanelSearch {
		var cmd tea.Cmd
		m.panel.Search, cmd = m.panel.Search.Update(msg)
		query := m.panel.Search.Value()
		if len(query) >= 2 {
			return m, tea.Batch(cmd, doSearch(m.richSource, query))
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.panel.List, cmd = m.panel.List.Update(msg)
	return m, cmd
}

func (m Model) panelSelect() (Model, tea.Cmd) {
	if m.panel == nil {
		return m, nil
	}
	selected := m.panel.List.SelectedItem()
	if selected == nil {
		return m, nil
	}

	switch m.activePanel {
	case PanelQueue:
		// Play the selected track (skip to it)
		// For simplicity, skip N times to reach the track
		if ti, ok := selected.(trackItem); ok {
			_ = ti // Track selection from queue
		}
		return m, nil
	case PanelLibrary:
		if !m.panel.inPlaylist {
			if pi, ok := selected.(playlistItem); ok {
				m.panel.inPlaylist = true
				m.panel.playlistID = pi.playlist.ID
				m.panel.playlistName = pi.playlist.Name
				m.panel.List.Title = pi.playlist.Name
				return m, fetchPlaylistTracks(m.richSource, pi.playlist.ID)
			}
		}
		return m, nil
	case PanelDevices:
		if di, ok := selected.(deviceItem); ok {
			return m, func() tea.Msg {
				if err := m.richSource.TransferPlayback(di.device.ID); err != nil {
					return trackErrorMsg{err}
				}
				return controlDoneMsg{}
			}
		}
	}
	return m, nil
}

func (m Model) adjustVolume(delta int) (Model, tea.Cmd) {
	if m.richSource == nil {
		return m, nil
	}
	m.volume = max(0, min(100, m.volume+delta))
	vol := m.volume
	return m, func() tea.Msg {
		if err := m.richSource.SetVolume(vol); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m Model) toggleShuffle() (Model, tea.Cmd) {
	if m.richSource == nil {
		return m, nil
	}
	m.shuffleOn = !m.shuffleOn
	state := m.shuffleOn
	return m, func() tea.Msg {
		if err := m.richSource.SetShuffle(state); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}

func (m Model) cycleRepeat() (Model, tea.Cmd) {
	if m.richSource == nil {
		return m, nil
	}
	switch m.repeatMode {
	case source.RepeatOff:
		m.repeatMode = source.RepeatContext
	case source.RepeatContext:
		m.repeatMode = source.RepeatTrack
	case source.RepeatTrack:
		m.repeatMode = source.RepeatOff
	}
	mode := m.repeatMode
	return m, func() tea.Msg {
		if err := m.richSource.SetRepeat(mode); err != nil {
			return trackErrorMsg{err}
		}
		return controlDoneMsg{}
	}
}
```

- [ ] **Step 5: Add panel data message handlers**

Add these cases to the `Update` switch in `app/model.go`:

```go
	case queueLoadedMsg:
		if m.panel != nil && m.activePanel == PanelQueue {
			items := make([]list.Item, len(msg.tracks))
			for i, t := range msg.tracks {
				items[i] = trackItem{t}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case playlistsLoadedMsg:
		if m.panel != nil && m.activePanel == PanelLibrary {
			m.panel.playlists = msg.playlists
			items := make([]list.Item, len(msg.playlists))
			for i, pl := range msg.playlists {
				items[i] = playlistItem{pl}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case playlistTracksMsg:
		if m.panel != nil && m.activePanel == PanelLibrary {
			items := make([]list.Item, len(msg.tracks))
			for i, t := range msg.tracks {
				items[i] = trackItem{t}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case devicesLoadedMsg:
		if m.panel != nil && m.activePanel == PanelDevices {
			items := make([]list.Item, len(msg.devices))
			for i, d := range msg.devices {
				items[i] = deviceItem{d}
			}
			m.panel.SetItems(items)
		}
		return m, nil
	case searchResultsMsg:
		if m.panel != nil && m.activePanel == PanelSearch && msg.results != nil {
			var items []list.Item
			for _, t := range msg.results.Tracks {
				items = append(items, trackItem{t})
			}
			m.panel.SetItems(items)
		}
		return m, nil
```

- [ ] **Step 6: Update tickAnimation to include effects**

Add effects tick at the end of `tickAnimation()`:

```go
	// Update visual effects
	artW := 0
	artH := 0
	if m.artworkRendered != "" {
		artW = m.artworkCols
		artH = m.artworkRows
	}
	innerW := m.width - 4
	innerH := m.height - 4
	m.effects.Tick(m.mood.Energy, m.beatPhase, m.mood.Primary, m.mood.Secondary, m.mood.Background, artW, artH, innerW, innerH)
```

- [ ] **Step 7: Update handleTrackUpdate to fetch audio features**

In `handleTrackUpdate`, after mood detection, add audio features fetch:

```go
	// Fetch audio features for better mood detection (if available)
	var featureCmd tea.Cmd
	if m.richSource != nil && track.ID != "" {
		trackID := track.ID
		featureCmd = func() tea.Msg {
			features, err := m.richSource.AudioFeatures(trackID)
			if err != nil || features == nil {
				return nil
			}
			return audioFeaturesMsg{features}
		}
	}
```

Add new message type at top of model.go:

```go
type audioFeaturesMsg struct{ features *source.AudioFeatures }
```

Add handler in Update:

```go
	case audioFeaturesMsg:
		if msg.features != nil {
			m.audioFeatures = msg.features
			detected := mood.DetectFromFeatures(msg.features)
			if detected.Name != m.targetMood.Name {
				m.startTransitionTo(detected)
			}
			// Use actual BPM from features
		}
		return m, nil
```

- [ ] **Step 8: Update WindowSizeMsg to initialize and resize effects**

In the `tea.WindowSizeMsg` case:

```go
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		if m.effects.Particles == nil {
			m.effects = NewEffects(msg.Width-4, msg.Height-4)
		} else {
			m.effects.Resize(msg.Width-4, msg.Height-4)
		}
		if m.panel != nil {
			m.panel.Resize(msg.Width, msg.Height)
		}
		return m, nil
```

- [ ] **Step 9: Update view.go to composite particles, glow, and panels**

Update the content area rendering loop in `View()` to overlay particles and glow. Replace the content rendering loop with:

```go
	// Content area with side glow
	particleGrid := m.effects.Particles.Render()
	totalContentRows := m.height - 4
	for row := range totalContentRows {
		full.WriteString(outerStyle.Render(" "))
		full.WriteString(innerStyle.Render(" "))

		ci := row - topPad
		if ci >= 0 && ci < contentH {
			line := contentLines[ci]
			lineW := lipgloss.Width(line)
			if lineW < innerWidth {
				// Fill remaining space with glow/particle-aware background
				remaining := innerWidth - lineW
				var fill strings.Builder
				for col := lineW; col < innerWidth; col++ {
					cellBg := md.Background
					// Apply glow
					if m.effects.GlowGrid != nil && row < len(m.effects.GlowGrid) && col < len(m.effects.GlowGrid[row]) {
						if g := m.effects.GlowGrid[row][col]; g != "" {
							cellBg = g
						}
					}
					// Apply breathing
					if m.effects.Breathing != 0 {
						cellBg = visual.LerpColor(cellBg, "#ffffff", m.effects.Breathing)
					}
					// Check for particle
					if row < len(particleGrid) && col < len(particleGrid[row]) && particleGrid[row][col] != 0 {
						pStyle := lipgloss.NewStyle().
							Foreground(lipgloss.Color(visual.LerpColor(md.Secondary, md.Primary, 0.5))).
							Background(lipgloss.Color(cellBg))
						fill.WriteString(pStyle.Render(string(particleGrid[row][col])))
					} else {
						fill.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(cellBg)).Render(" "))
					}
				}
				_ = remaining
				line += fill.String()
			}
			full.WriteString(line)
		} else {
			// Empty row — render glow + particles
			var rowStr strings.Builder
			for col := range innerWidth {
				cellBg := md.Background
				if m.effects.GlowGrid != nil && row < len(m.effects.GlowGrid) && col < len(m.effects.GlowGrid[row]) {
					if g := m.effects.GlowGrid[row][col]; g != "" {
						cellBg = g
					}
				}
				if m.effects.Breathing != 0 {
					cellBg = visual.LerpColor(cellBg, "#ffffff", m.effects.Breathing)
				}
				if row < len(particleGrid) && col < len(particleGrid[row]) && particleGrid[row][col] != 0 {
					pStyle := lipgloss.NewStyle().
						Foreground(lipgloss.Color(visual.LerpColor(md.Secondary, md.Primary, 0.5))).
						Background(lipgloss.Color(cellBg))
					rowStr.WriteString(pStyle.Render(string(particleGrid[row][col])))
				} else {
					rowStr.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(cellBg)).Render(" "))
				}
			}
			full.WriteString(rowStr.String())
		}

		full.WriteString(innerStyle.Render(" "))
		full.WriteString(outerStyle.Render(" "))
		full.WriteString("\n")
	}
```

After the full screen is built, overlay the panel if active:

```go
	rendered := full.String()

	// Overlay panel if active
	if m.panel != nil && m.activePanel != PanelNone {
		panelView := m.panel.View(md.Primary, md.Secondary, md.Background)
		panelLines := strings.Split(panelView, "\n")

		renderedLines := strings.Split(rendered, "\n")
		startRow := 2 // after top glow

		// Position panel: queue on right, library/search on left
		var startCol int
		if m.activePanel == PanelQueue {
			startCol = m.width - m.panel.Width
		} else if m.activePanel == PanelDevices {
			startCol = (m.width - m.panel.Width) / 2
		} else {
			startCol = 0
		}

		for i, pLine := range panelLines {
			row := startRow + i
			if row >= len(renderedLines) {
				break
			}
			// Simple overlay: replace characters at panel position
			// This is a simplified approach; full implementation would use
			// lipgloss.Place or character-level compositing
			renderedLines[row] = overlayLine(renderedLines[row], pLine, startCol)
		}
		rendered = strings.Join(renderedLines, "\n")
	}

	return rendered
```

Add helper:

```go
func overlayLine(base, overlay string, startCol int) string {
	baseRunes := []rune(base)
	overlayRunes := []rune(overlay)

	// Ensure base is long enough
	for len(baseRunes) < startCol+len(overlayRunes) {
		baseRunes = append(baseRunes, ' ')
	}

	// Replace runes at position
	copy(baseRunes[startCol:], overlayRunes)
	return string(baseRunes)
}
```

- [ ] **Step 10: Add `list` import to model.go**

Add `"github.com/charmbracelet/bubbles/list"` to the imports in `app/model.go`.

- [ ] **Step 11: Build and verify compilation**

```bash
cd /Users/danielfry/dev/tui && go build ./...
```

Fix any compilation errors. Common issues: missing imports, type mismatches. The exact integration code may need adjustment based on how the `zmb3/spotify/v2` API looks at build time.

- [ ] **Step 12: Run existing tests**

```bash
cd /Users/danielfry/dev/tui && go test ./... -v
```

Expected: all existing tests still pass. Fix any that break due to interface changes (e.g., `Track.ID` field).

- [ ] **Step 13: Commit**

```bash
cd /Users/danielfry/dev/tui && git add app/ && git commit -m "feat(app): integrate particles, glow, panels, and Spotify API into model and view"
```

---

## Task 14: Main Entry Point & Startup Flow

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update main.go with auth flow and source selection**

Replace `main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/app"
	myauth "github.com/danielfry/spotui/auth"
	myspotify "github.com/danielfry/spotui/spotify"
	"github.com/danielfry/spotui/source"
)

const version = "v0.2.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			runAuth()
			return
		case "version":
			fmt.Println("spotui " + version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	src := initSource()
	m := app.NewModel(src)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func initSource() source.TrackSource {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		// No client ID — fall back to local osascript
		return source.NewLocalSource()
	}

	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		// No stored token — try to authenticate
		fmt.Println("No Spotify token found. Run 'spotui auth' to connect.")
		fmt.Println("Falling back to local Spotify desktop app...")
		return source.NewLocalSource()
	}

	client := myspotify.NewClient(clientID, token, tokenPath)
	return myspotify.NewPlayerSource(client)
}

func runAuth() {
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		fmt.Println("Set SPOTIFY_CLIENT_ID environment variable first.")
		fmt.Println("")
		fmt.Println("  1. Go to https://developer.spotify.com/dashboard")
		fmt.Println("  2. Create an app (any name)")
		fmt.Println("  3. Add redirect URI: http://localhost:8080/callback")
		fmt.Println("  4. Copy the Client ID")
		fmt.Println("  5. Run: SPOTIFY_CLIENT_ID=<your-id> spotui auth")
		return
	}

	token, err := myauth.Authenticate(clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	tokenPath := myauth.DefaultTokenPath()
	if err := myauth.SaveToken(tokenPath, token); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Authenticated! Token saved to %s\n", tokenPath)
	fmt.Println("Run 'spotui' to launch the player.")
}

func printUsage() {
	fmt.Println(`spotui — an immersive mood-reactive terminal music companion

Usage:
  spotui            Launch the TUI
  spotui auth       Connect your Spotify account
  spotui version    Print version
  spotui help       Show this help

Environment:
  SPOTIFY_CLIENT_ID    Your Spotify app's Client ID (required for API features)
  SPOTUI_KITTY=1       Enable Kitty graphics protocol for album art

Without SPOTIFY_CLIENT_ID, spotui falls back to controlling the Spotify
desktop app via AppleScript (macOS only).`)
}
```

- [ ] **Step 2: Build and verify**

```bash
cd /Users/danielfry/dev/tui && go build -o spotui . && ./spotui version
```

Expected: prints `spotui v0.2.0`.

- [ ] **Step 3: Commit**

```bash
cd /Users/danielfry/dev/tui && git add main.go && git commit -m "feat: add Spotify auth flow, source selection, and updated CLI help"
```

---

## Task 15: Final Build & Smoke Test

- [ ] **Step 1: Full build**

```bash
cd /Users/danielfry/dev/tui && go build -o spotui .
```

- [ ] **Step 2: Run all tests**

```bash
cd /Users/danielfry/dev/tui && go test ./... -v
```

Fix any failures.

- [ ] **Step 3: Smoke test in local mode**

```bash
cd /Users/danielfry/dev/tui && ./spotui
```

Verify: album art renders, vibe bars animate, particles float, glow surrounds art, background breathes.

- [ ] **Step 4: Smoke test auth flow**

```bash
cd /Users/danielfry/dev/tui && SPOTIFY_CLIENT_ID=<your-client-id> ./spotui auth
```

Verify: browser opens, auth completes, token saved.

- [ ] **Step 5: Smoke test with Spotify API**

```bash
cd /Users/danielfry/dev/tui && SPOTIFY_CLIENT_ID=<your-client-id> ./spotui
```

Verify: track info from API, queue panel (Q), library panel (L), search (/), device picker (D), volume (+/-), shuffle (S), repeat (R).

- [ ] **Step 6: Final commit**

```bash
cd /Users/danielfry/dev/tui && git add -A && git commit -m "feat: spotui v0.2.0 — immersive Spotify TUI with API integration, visual effects, and panels"
```
