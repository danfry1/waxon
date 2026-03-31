package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ToastType controls the visual style of a toast notification.
type ToastType int

const (
	ToastSuccess ToastType = iota
	ToastError
	ToastInfo
)

// clearToastMsg dismisses the current toast.
type clearToastMsg struct{}

const toastDuration = 3 * time.Second

// Toast is a floating notification rendered in the top-right corner.
type Toast struct {
	message   string
	detail    string
	toastType ToastType
	visible   bool
}

// Show makes the toast visible with the given content.
func (t *Toast) Show(message, detail string, tt ToastType) {
	// Truncate to first line and max 60 runes to prevent layout breaks.
	// Uses rune-level slicing to avoid splitting multi-byte UTF-8 characters.
	if idx := strings.IndexAny(message, "\n\r{"); idx >= 0 {
		message = message[:idx]
	}
	if runes := []rune(message); len(runes) > 60 {
		message = string(runes[:57]) + "..."
	}
	if idx := strings.IndexAny(detail, "\n\r"); idx >= 0 {
		detail = detail[:idx]
	}
	t.message = message
	t.detail = detail
	t.toastType = tt
	t.visible = true
}

// Hide clears the toast.
func (t *Toast) Hide() {
	t.visible = false
}

// Visible returns whether the toast is currently shown.
func (t Toast) Visible() bool {
	return t.visible
}

// scheduleAutoDismiss returns a Cmd that will send clearToastMsg after the toast duration.
func scheduleAutoDismiss() tea.Cmd {
	return tea.Tick(toastDuration, func(t time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

// icon returns the prefix icon for the toast type.
func (t Toast) icon() string {
	switch t.toastType {
	case ToastSuccess:
		return "✓"
	case ToastError:
		return "✗"
	case ToastInfo:
		return "♪"
	default:
		return "•"
	}
}

// borderColor returns the lipgloss border color for the toast type.
func (t Toast) borderColor() lipgloss.Color {
	switch t.toastType {
	case ToastSuccess:
		return ColorAccent // #1DB954 green
	case ToastError:
		return ColorError // #E22134 red
	default:
		return ColorTextDim
	}
}

// View renders the toast as a floating notification card.
func (t Toast) View(screenWidth int) string {
	if !t.visible || t.message == "" {
		return ""
	}

	bc := t.borderColor()

	iconStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	msgStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	var sb strings.Builder
	sb.WriteString(iconStyle.Render(t.icon()) + "  " + msgStyle.Render(t.message))
	if t.detail != "" {
		sb.WriteString("\n   " + detailStyle.Render(t.detail))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(0, 1).
		Render(sb.String())
}

// Overlay composites the toast box onto the top-right corner of the rendered view.
func (t Toast) Overlay(base string, screenWidth int) string {
	box := t.View(screenWidth)
	if box == "" {
		return base
	}

	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	boxW := lipgloss.Width(boxLines[0])

	// Position: row 1, right-aligned with 2-col margin
	startRow := 1
	startCol := screenWidth - boxW - 2
	if startCol < 0 {
		startCol = 0
	}

	for i, bl := range boxLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		// ANSI-aware truncate: keep the left portion of the base line,
		// then append the overlay (which extends to the right edge).
		left := ansi.Truncate(baseLines[row], startCol, "")
		// Pad if base line was shorter than startCol
		leftW := lipgloss.Width(left)
		if leftW < startCol {
			left += strings.Repeat(" ", startCol-leftW)
		}
		// Restore the right portion of the base line after the toast
		// so pane borders aren't clipped.
		right := stripLeft(baseLines[row], startCol+boxW)
		baseLines[row] = left + "\033[0m" + bl + "\033[0m" + right
	}

	return strings.Join(baseLines, "\n")
}

// stripLeft removes the first n visible columns from an ANSI string,
// preserving any ANSI sequences that affect the remaining text.
func stripLeft(s string, n int) string {
	totalW := lipgloss.Width(s)
	if n >= totalW {
		return ""
	}
	// Truncate to the right portion by cutting from the full width
	// ansi.TruncateLeft(s, n, "") drops the first n visible columns.
	return ansi.TruncateLeft(s, n, "")
}
