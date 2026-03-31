package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/waxon/app"
	myauth "github.com/danielfry/waxon/auth"
	"github.com/danielfry/waxon/config"
	myspotify "github.com/danielfry/waxon/spotify"
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
		case "setup":
			runSetup()
			return
		case "auth":
			runAuth()
			return
		case "debug":
			runDebug()
			return
		case "version":
			fmt.Println("waxon " + version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

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
		cleanup()
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No Spotify token found. Run 'waxon setup' to connect your account.")
		} else {
			fmt.Fprintf(os.Stderr, "Failed to read token: %v\n", err)
			fmt.Println("Your token file may be corrupted. Run 'waxon auth' to re-authenticate.")
		}
		os.Exit(1) //nolint:gocritic // cleanup called above
	}

	cp := myspotify.NewClient(clientID, token, tokenPath)
	src := myspotify.NewPlayerSource(cp)

	m := app.NewModel(src)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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

func runSetup() {
	fmt.Println("")
	fmt.Println("  Welcome to waxon! Let's connect your Spotify account.")

	authenticate()

	fmt.Println("")
	fmt.Println("  You're all set! Run 'waxon' to start.")
	fmt.Println("")
}

func runAuth() {
	tokenPath := authenticate()
	fmt.Printf("Authenticated! Token saved to %s\n", tokenPath)
	fmt.Println("Run 'waxon' to launch.")
}

// authenticate performs the OAuth flow and saves the token. Returns the token path.
func authenticate() string {
	clientID := resolveClientID()
	if clientID == "" {
		clientID = myauth.DefaultClientID
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

	// Persist the client ID so subsequent launches use the same one.
	if err := config.Save(config.Config{ClientID: clientID}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save config: %v\n", err)
	}

	return tokenPath
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
  waxon setup    First-time setup wizard
  waxon auth     Re-authorize your Spotify account
  waxon version  Print version
  waxon help     Show this help

Environment:
  SPOTIFY_CLIENT_ID  Override the saved Client ID
  WAXON_LOG       Path to debug log file (e.g. /tmp/waxon.log)`)
}
