package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/muesli/cancelreader"
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
	"playlist-modify-public",
	"playlist-modify-private",
	"user-library-read",
	"user-library-modify",
	"user-read-recently-played",
	"user-follow-read",
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
			// Not this login's redirect (a stale tab, or something probing
			// the port): refuse it without ending the login, which may still
			// complete here or by paste.
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			slog.Warn("ignored OAuth callback with mismatched state")
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

// RedirectURI is the callback URL to register in the Spotify dashboard for a
// personal client ID (exported for the setup guide).
func RedirectURI(clientID string) string { return redirectURI(clientID) }

// OpenBrowser opens url in the user's default browser (best effort).
func OpenBrowser(url string) error { return openBrowser(url) }

// redirectURI returns the exact redirect URI sent to Spotify for the given
// client ID. Spotify matches this string against the dashboard registration;
// for the ncspot client ID the registered URI is the port-less loopback
// "http://127.0.0.1/login", so Spotify accepts our fixed port via RFC 8252
// loopback port flexibility — but the host must be the 127.0.0.1 IP literal
// (not "localhost") and the path must match exactly.
func redirectURI(clientID string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", DefaultCallbackPort, callbackPath(clientID))
}

// Login is one browser sign-in attempt. Show URL to the user, then Wait: it
// returns once Spotify redirects to the local callback server or the user
// Pastes the address the browser landed on (for when the browser can't reach
// this machine, e.g. over SSH).
type Login struct {
	URL string
	// Listening is false when the callback port was taken; the login then
	// completes only by Paste.
	Listening bool

	cfg      *oauth2.Config
	verifier string
	state    string
	codeCh   chan string
	errCh    chan error

	mu     sync.Mutex // guards server: Close may race Wait's own close
	server *http.Server
}

// BeginLogin prepares a sign-in for clientID and starts the callback server.
// Close it when done (Wait does on success).
func BeginLogin(clientID string) (*Login, error) {
	cfg := SpotifyOAuthConfig(clientID, redirectURI(clientID))
	verifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}
	state, err := generateVerifier() // random state for CSRF protection
	if err != nil {
		return nil, err
	}
	l := &Login{
		cfg:      cfg,
		verifier: verifier,
		state:    state,
		codeCh:   make(chan string, 1),
		errCh:    make(chan error, 1),
	}
	l.URL = cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", challengeFromVerifier(verifier)),
	)
	addr := fmt.Sprintf("127.0.0.1:%d", DefaultCallbackPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Pasting the redirect still works without the callback server.
		slog.Warn("callback listener unavailable; paste-only login", "addr", addr, "error", err)
		return l, nil
	}
	l.Listening = true
	l.server = &http.Server{
		Handler:           callbackHandler(callbackPath(clientID), state, l.codeCh, l.errCh),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = l.server.Serve(listener) }()
	return l, nil
}

// Paste offers the address the browser landed on. An error says what's wrong
// with it, so the user can paste again; a refusal on Spotify's page also ends
// Wait.
func (l *Login) Paste(raw string) error {
	code, err := codeFromRedirect(strings.TrimSpace(raw), l.state)
	var denied deniedError
	if errors.As(err, &denied) {
		select {
		case l.errCh <- err:
		default:
		}
		return err
	}
	if err != nil {
		return err
	}
	select {
	case l.codeCh <- code:
	default:
	}
	return nil
}

// Wait blocks until the sign-in completes, fails, ctx ends, or five minutes
// pass, and exchanges the code for a token.
func (l *Login) Wait(ctx context.Context) (*oauth2.Token, error) {
	defer l.Close()
	var code string
	select {
	case code = <-l.codeCh:
	case err := <-l.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, errors.New("timed out waiting for Spotify authorization (5 minutes)")
	}
	// Bound the token exchange so a stalled token endpoint can't hang the
	// sign-in indefinitely.
	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	token, err := l.cfg.Exchange(exchangeCtx, code,
		oauth2.SetAuthURLParam("code_verifier", l.verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	return token, nil
}

// Close stops the callback server, letting a response in progress (the
// "Connected" page) finish without making the caller wait for it. Safe to
// call more than once, from any goroutine.
func (l *Login) Close() {
	l.mu.Lock()
	srv := l.server
	l.server = nil
	l.mu.Unlock()
	if srv != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()
	}
}

// Authenticate runs a Login in the terminal: it prints the URL (opening the
// browser unless over SSH) and accepts a pasted redirect on stdin.
func Authenticate(clientID string) (*oauth2.Token, error) {
	l, err := BeginLogin(clientID)
	if err != nil {
		return nil, err
	}
	stopPaste, canPaste := readPastedRedirect(l)
	defer stopPaste()
	if !l.Listening && !canPaste {
		l.Close()
		return nil, fmt.Errorf("listen on 127.0.0.1:%d failed (is another instance running?)", DefaultCallbackPort)
	}

	if IsRemoteSession() {
		fmt.Printf("\n  This looks like an SSH session. Open this URL in a browser on any machine:\n\n    %s\n\n", l.URL)
	} else {
		fmt.Printf("\n  Opening browser for Spotify login...\n")
		if browserErr := openBrowser(l.URL); browserErr != nil {
			fmt.Printf("  Could not open browser automatically.\n")
		}
		fmt.Printf("  If it doesn't open, visit this URL:\n\n    %s\n\n", l.URL)
	}
	if canPaste {
		fmt.Printf("  If the browser can't reach this machine, the page after you approve won't load.\n")
		fmt.Printf("  That's expected: copy its address from the address bar and paste it here.\n\n")
	}
	if !l.Listening {
		fmt.Printf("  (Couldn't listen on 127.0.0.1:%d, so paste the address when you get there.)\n", DefaultCallbackPort)
	}
	fmt.Printf("  Waiting for Spotify authorization...\n")
	return l.Wait(context.Background())
}

// NormalizeClientID trims and validates a pasted client ID: Spotify issues
// 32 lowercase hex characters.
func NormalizeClientID(s string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(s))
	if len(id) != 32 {
		return "", fmt.Errorf("a client ID is 32 characters (got %d)", len(id))
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", errors.New("a client ID is hexadecimal (0-9, a-f)")
		}
	}
	return id, nil
}

// IsRemoteSession reports whether waxon is running over SSH, where a browser
// opened here (if any) isn't the one the user is looking at.
func IsRemoteSession() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// readPastedRedirect reads lines from stdin, passing a pasted redirect URL to
// l as an alternative to the callback server.
// The returned stop cancels the read: waxon may start the TUI next, and a
// reader left blocked on stdin would swallow its first keystrokes. ok is
// false when stdin can't be read this way, and pasting isn't offered.
//
// Windows is left on the callback-only flow: cancelreader puts the console in
// raw mode there (no echo, no line editing, Ctrl-C delivered as a byte).
func readPastedRedirect(l *Login) (stop func(), ok bool) {
	if runtime.GOOS == "windows" {
		return func() {}, false
	}
	r, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return func() {}, false
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			err := l.Paste(line)
			var denied deniedError
			if errors.As(err, &denied) {
				return
			}
			if err != nil {
				fmt.Printf("  %v. Paste the full address, starting with http://127.0.0.1\n", err)
				continue
			}
			return
		}
	}()
	return func() {
		// Wait for the reader to exit only when the cancel took effect; the
		// fallback reader can't interrupt a blocked read, and waiting would
		// hang until a key is pressed.
		if r.Cancel() {
			<-done
		}
		_ = r.Close()
	}, true
}

// deniedError is a redirect carrying Spotify's refusal (e.g. the user
// pressed Cancel), which ends the login rather than asking for another paste.
type deniedError struct{ reason string }

func (e deniedError) Error() string { return "auth denied: " + e.reason }

// codeFromRedirect extracts the authorization code from a pasted redirect
// URL, checking its state like the callback handler does.
func codeFromRedirect(raw, state string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw // some address bars drop the scheme when copied
	}
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return "", errors.New("that doesn't look like the redirect address")
	}
	q := u.Query()
	if q.Get("state") != state {
		return "", errors.New("that address is from a different login attempt")
	}
	if e := q.Get("error"); e != "" {
		return "", deniedError{e}
	}
	code := q.Get("code")
	if code == "" {
		return "", errors.New("the address has no authorization code")
	}
	return code, nil
}

func openBrowser(url string) error {
	// WAXON_NO_BROWSER=1 skips launching a browser (SSH sessions, recordings);
	// callers always print the URL as well.
	if os.Getenv("WAXON_NO_BROWSER") != "" {
		return errors.New("WAXON_NO_BROWSER is set")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		// Not "cmd /c start": cmd.exe splits the URL at its & characters.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to open browser", "error", err)
		return err
	}
	return nil
}
