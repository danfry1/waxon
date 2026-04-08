package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

// DefaultClientID is the ncspot client ID (Extended Quota Mode).
// This is the same approach used by spotify-player and other open-source
// Spotify terminal clients — it avoids requiring users to create their
// own developer app.
const DefaultClientID = "d420a117a32841c2b3474932e49fb54b"

var spotifyScopes = []string{
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-read-private",
	"playlist-read-collaborative",
	"user-library-read",
	"user-library-modify",
	"user-read-recently-played",
}

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

func generateVerifier() (string, error) {
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func challengeFromVerifier(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// callbackHandler builds the HTTP handler used by the OAuth callback server.
// It validates the state parameter, extracts the authorization code, and sends
// results on the provided channels. The path parameter sets which URL path the
// handler listens on (e.g. "/login" or "/callback").
func callbackHandler(path, expectedState string, codeCh chan<- string, errCh chan<- error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != expectedState {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			select {
			case errCh <- errors.New("OAuth state mismatch (possible CSRF)"):
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			http.Error(w, "Auth failed: "+errMsg, http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("auth denied: %s", errMsg):
			default:
			}
			return
		}
		fmt.Fprint(w, "<html><body><h2>Connected to Spotify!</h2><p>You can close this tab.</p></body></html>")
		select {
		case codeCh <- code:
		default:
		}
	})
	return mux
}

// DefaultCallbackPort is the fixed port used for the OAuth callback server.
// Users registering their own Spotify app should add
// http://127.0.0.1:27228/callback as a redirect URI.
const DefaultCallbackPort = 27228

// callbackPath returns the OAuth redirect path for the given client ID.
// The default ncspot client ID has "/login" registered in Spotify's dashboard,
// while custom client IDs use "/callback" (matching waxon's documented setup).
func callbackPath(clientID string) string {
	if clientID == DefaultClientID {
		return "/login"
	}
	return "/callback"
}

func Authenticate(clientID string) (*oauth2.Token, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", DefaultCallbackPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w (is another instance running?)", addr, err)
	}
	path := callbackPath(clientID)
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d%s", DefaultCallbackPort, path)

	cfg := SpotifyOAuthConfig(clientID, redirectURL)
	verifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}
	challenge := challengeFromVerifier(verifier)
	state, err := generateVerifier() // random state for CSRF protection
	if err != nil {
		return nil, err
	}

	authURL := cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
	)

	fmt.Printf("\nOpening browser for Spotify login...\n")
	if browserErr := openBrowser(authURL); browserErr != nil {
		fmt.Printf("Could not open browser automatically.\n")
	}
	fmt.Printf("If it doesn't open, visit this URL:\n\n  %s\n\n", authURL)
	fmt.Printf("Waiting for Spotify authorization...\n")

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	server := &http.Server{Handler: callbackHandler(path, state, codeCh, errCh)}
	go func() { _ = server.Serve(listener) }()

	var code string
	select {
	case code = <-codeCh:
	case authErr := <-errCh:
		_ = server.Close()
		return nil, authErr
	case <-time.After(5 * time.Minute):
		_ = server.Close()
		return nil, errors.New("timed out waiting for Spotify authorization (5 minutes)")
	}

	_ = server.Shutdown(context.Background())

	token, err := cfg.Exchange(context.Background(), code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	return token, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to open browser", "error", err)
		return err
	}
	return nil
}
