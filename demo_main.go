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

// demoUsage is the help line for `waxon demo` (only in demo builds).
const demoUsage = "  waxon demo     Launch in demo mode, on built-in sample data\n"

// runDemo launches the TUI against the demo source. "waxon demo <cmd> ..."
// runs a CLI subcommand against the same demo data instead (for recordings
// and manual testing without a Spotify account).
func runDemo() {
	src := demo.NewDemoSource()
	if len(os.Args) > 2 && cli.IsCommand(os.Args[2]) {
		r := &cli.Runner{Src: src, Out: os.Stdout, Err: os.Stderr}
		os.Exit(r.Run(context.Background(), os.Args[2:]))
	}

	if len(os.Args) > 2 && os.Args[2] == "setup" {
		// The guided setup, against stand-ins (see demo_setup.go).
		deps, cleanup := demoSetupDeps()
		defer cleanup()
		s := app.NewSetup(deps, app.SetupFull)
		if _, err := tea.NewProgram(s, tea.WithAltScreen()).Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("  Starting waxon in demo mode...")
	fmt.Println("")

	// Demo mode ignores the user's config (recordings must be reproducible)
	// and never persists :theme changes. WAXON_ART still applies, for trying
	// kitty art without a Spotify account.
	configureArt(os.Getenv("WAXON_ART"), "WAXON_ART")
	m := app.NewModel(src).WithOptions(app.Options{Settings: app.Settings{MediaKeys: true}})
	p := tea.NewProgram(m, programOptions()...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
