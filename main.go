package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/app"
	"github.com/danielfry/spotui/source"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			fmt.Println("spotui auth — Spotify API integration coming soon")
			fmt.Println("For now, spotui works with the Spotify desktop app automatically.")
			return
		case "version":
			fmt.Println("spotui v0.1.0")
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}
	src := source.NewLocalSource()
	m := app.NewModel(src)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`spotui — a mood-reactive terminal music companion

Usage:
  spotui            Launch the TUI
  spotui auth       Connect Spotify API (coming soon)
  spotui version    Print version
  spotui help       Show this help`)
}
