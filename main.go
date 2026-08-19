package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/app"
	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/cli"
	"github.com/danfry1/waxon/config"
	myspotify "github.com/danfry1/waxon/spotify"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	cleanup := initLogging()
	defer cleanup()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			opts, err := parseAuthFlags(os.Args[2:])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				cleanup()
				os.Exit(2) //nolint:gocritic // cleanup called above
			}
			runAuthWith(newOnboarding(), opts)
			return
		case "debug":
			runDebug()
			return
		case "version", "--version", "-v":
			fmt.Println("waxon " + version)
			return
		case "demo":
			runDemo()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		case "themes":
			for _, name := range app.ThemeNames() {
				fmt.Println(name)
			}
			return
		case "config":
			fmt.Println(config.DefaultPath())
			return
		}
		if cli.IsCommand(os.Args[1]) {
			src, ok := newSource()
			if !ok {
				cleanup()
				os.Exit(1) //nolint:gocritic // cleanup called above
			}
			r := &cli.Runner{Src: src, Out: os.Stdout, Err: os.Stderr}
			code := r.Run(context.Background(), os.Args[1:])
			cleanup()
			os.Exit(code) //nolint:gocritic // cleanup called above
		}
	}

	src, ok := newSource()
	if !ok {
		cleanup()
		os.Exit(1) //nolint:gocritic // cleanup called above
	}

	opts := applyUserConfig()
	opts.SharedClientID = effectiveClientID() == myauth.DefaultClientID
	m := app.NewModel(src).WithOptions(opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// applyUserConfig reads theme/colour/key settings from the config file,
// applies the theme and art mode globally, and returns the Model options.
// Problems are reported on stderr and fall back to defaults rather than
// refusing to start — a typo in config.json shouldn't lock anyone out.
func applyUserConfig() app.Options {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v (using defaults)\n", err)
	}
	var opts app.Options

	overrides, err := app.PaletteFromMap(cfg.Colors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config colors: %v (ignored)\n", err)
		overrides = app.Palette{}
	}
	opts.ColorOverrides = overrides
	palette, err := app.ResolveTheme(cfg.Theme, overrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v (using %s)\n", err, app.DefaultThemeName)
		palette, _ = app.ResolveTheme(app.DefaultThemeName, overrides)
	}
	app.ApplyTheme(palette)
	app.SetArtMode(app.DetectArtMode())
	if dir := app.DefaultImageCacheDir(); dir != "" {
		app.SetImageCacheDir(dir)
		go app.PruneImageCache()
	}

	if len(cfg.Keys) > 0 {
		km, err := app.KeyMapFromOverrides(cfg.Keys)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: config keys: %v (using defaults)\n", err)
		} else {
			opts.Keys = &km
		}
	}
	opts.SaveTheme = func(name string) error {
		return config.Save(config.Config{Theme: name})
	}
	return opts
}

// newSource builds the Spotify-backed source from the saved token. On failure
// it prints guidance and returns ok=false (the caller exits).
func newSource() (*myspotify.PlayerSource, bool) {
	clientID := resolveClientID()
	if clientID == "" {
		clientID = myauth.DefaultClientID
	}

	// Warn if the token was issued for a different client ID.
	if saved, err := config.Load(); err == nil && saved.ClientID != "" && saved.ClientID != clientID {
		fmt.Fprintf(os.Stderr, "Warning: current client ID differs from the one used during auth.\n")
		fmt.Fprintf(os.Stderr, "Run 'waxon auth' to re-authenticate, or set SPOTIFY_CLIENT_ID=%s\n", saved.ClientID)
	}

	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// First run: go straight into setup instead of bouncing the user
			// to a command they haven't heard of yet.
			o := newOnboarding()
			if o.interactive {
				fmt.Fprintln(o.out, "  Welcome to waxon — let's connect your Spotify account.")
				runAuthFlow(o, authOptions{}, true)
				token, err = myauth.LoadToken(tokenPath)
			}
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "No Spotify token found. Run 'waxon auth' to connect your account.")
		} else {
			fmt.Fprintf(os.Stderr, "Failed to read token: %v\n", err)
			fmt.Fprintln(os.Stderr, "Your token file may be corrupted. Run 'waxon auth' to re-authenticate.")
		}
		return nil, false
	}

	cp := myspotify.NewClient(clientID, token, tokenPath)
	return myspotify.NewPlayerSource(cp), true
}

// effectiveClientID is the client ID waxon will use: the configured one, or
// the bundled shared ID when none is set.
func effectiveClientID() string {
	if id := resolveClientID(); id != "" {
		return id
	}
	return myauth.DefaultClientID
}

// resolveClientID returns the Client ID from the env var (takes priority)
// or from the saved config file. Returns "" if neither is set.
func resolveClientID() string {
	if id := os.Getenv("SPOTIFY_CLIENT_ID"); id != "" {
		return id
	}
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	return cfg.ClientID
}

// authenticateWith runs the OAuth flow for clientID and saves the token and
// client ID.
func authenticateWith(clientID string) {
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

	// Persist the client ID so subsequent launches use the same one.
	if err := config.Save(config.Config{ClientID: clientID}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}
}

func runDebug() {
	clientID := resolveClientID()
	if clientID == "" {
		clientID = myauth.DefaultClientID
		fmt.Println("Client ID: (default)")
	} else {
		fmt.Printf("Client ID: %s\n", clientID)
	}
	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		fmt.Println("No token:", err)
		return
	}
	cp := myspotify.NewClient(clientID, token, tokenPath)
	src := myspotify.NewPlayerSource(cp)

	ctx := context.Background()
	ps, err := src.CurrentPlayback(ctx)
	if err != nil {
		fmt.Println("CurrentPlayback error:", err)
		return
	}
	if ps == nil || ps.Track == nil {
		fmt.Println("No track playing")
		return
	}
	track := ps.Track
	fmt.Printf("Track: %s - %s (ArtistID=%s, AlbumID=%s)\n", track.Name, track.Artist, track.ArtistID, track.AlbumID)

	if track.ArtistID != "" {
		fmt.Printf("\nGetArtist(%s)...\n", track.ArtistID)
		page, err := src.GetArtist(ctx, track.ArtistID)
		if err != nil {
			fmt.Println("ERROR:", err)
		} else {
			fmt.Printf("Artist: %s, Genres: %v, Tracks: %d\n", page.Name, page.Genres, len(page.Tracks))
		}
	}

	if track.AlbumID != "" {
		fmt.Printf("\nGetAlbum(%s)...\n", track.AlbumID)
		page, err := src.GetAlbum(ctx, track.AlbumID)
		if err != nil {
			fmt.Println("ERROR:", err)
		} else {
			fmt.Printf("Album: %s — %s, Tracks: %d\n", page.Name, page.Artist, len(page.Tracks))
		}
	}
}

// initLogging configures the global slog logger and returns a cleanup function
// that closes the log file. The caller must defer the cleanup.
// If WAXON_LOG is set to a file path, debug-level logs are written there;
// otherwise logging is disabled.
func initLogging() func() {
	logPath := os.Getenv("WAXON_LOG")
	if logPath == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() {}
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open log file: %v\n", err)
		return func() {}
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	slog.Info("waxon starting", "version", version)
	return func() { _ = f.Close() }
}

func printUsage() {
	fmt.Println(`waxon — vim-modal Spotify terminal client

Usage:
  waxon          Launch the TUI
  waxon auth     Connect your Spotify account (guided; --own | --shared | --client-id ID)
  waxon demo     Launch in demo mode (requires -tags demo build)
  waxon version  Print version
  waxon themes   List colour themes (set with "theme" in config.json or :theme)
  waxon config   Print the config file path
  waxon help     Show this help

` + cli.Usage() + `

Environment:
  SPOTIFY_CLIENT_ID  Override the saved Client ID
  WAXON_LOG          Path to debug log file (e.g. /tmp/waxon.log)`)
}
