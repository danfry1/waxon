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
		return source.NewLocalSource()
	}

	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
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

	fmt.Printf("Authenticated! Token saved to %s\n", tokenPath)
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
