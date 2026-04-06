//go:build demo

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/app"
	"github.com/danfry1/waxon/demo"
)

func runDemo() {
	fmt.Println("  Starting waxon in demo mode...")
	fmt.Println("")

	src := demo.NewDemoSource()
	m := app.NewModel(src)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
