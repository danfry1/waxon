//go:build demo

package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/app"
	"github.com/danfry1/waxon/cli"
	"github.com/danfry1/waxon/demo"
)

// runDemo launches the TUI against the demo source. "waxon demo <cmd> ..."
// runs a CLI subcommand against the same demo data instead (for recordings
// and manual testing without a Spotify account).
func runDemo() {
	src := demo.NewDemoSource()
	if len(os.Args) > 2 && cli.IsCommand(os.Args[2]) {
		r := &cli.Runner{Src: src, Out: os.Stdout, Err: os.Stderr}
		os.Exit(r.Run(context.Background(), os.Args[2:]))
	}

	fmt.Println("  Starting waxon in demo mode...")
	fmt.Println("")

	// Demo mode ignores the user's config (recordings must be reproducible)
	// and never persists :theme changes.
	app.SetArtMode(app.DetectArtMode())
	m := app.NewModel(src)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
