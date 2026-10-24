package app

import (
	"fmt"
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

// clearToastMsg dismisses the toast if it has been up for toastDuration
// (at is when the timer fired; zero dismisses it now). A timer left over
// from an earlier toast doesn't cut a newer one short.
type clearToastMsg struct{ at time.Time }

const toastDuration = 3 * time.Second

// Toast is a floating notification rendered in the top-right corner.
type Toast struct {
	message   string
	detail    string
	toastType ToastType
	visible   bool
	shownAt   time.Time
	// meter makes this the volume toast, drawn from volume (set by the
	// model each frame).
	meter  bool
	volume volumeShow
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
	t.meter = false
	t.shownAt = time.Now()
}

// ShowVolume shows (or keeps up) the volume meter.
func (t *Toast) ShowVolume() {
	t.message, t.detail, t.toastType = "Volume", "", ToastInfo
	t.visible, t.meter, t.shownAt = true, true, time.Now()
}

// Expired reports whether a dismiss timer that fired at at should hide the
// toast.
func (t Toast) Expired(at time.Time) bool {
	return at.IsZero() || at.Sub(t.shownAt) >= toastDuration-50*time.Millisecond
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
		return clearToastMsg{at: t}
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
	if t.meter {
		return t.meterView()
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

// meterWidth is the volume bar's length in cells.
const meterWidth = 16

// meterView is the volume toast: the accepted level in the accent, the
// part still on its way dim, and a frame that glows faintly as the last
// level lands.
func (t Toast) meterView() string {
	v := t.volume
	cells := func(level int) int { return (max(0, min(100, level))*meterWidth + 50) / 100 }
	solid, target := cells(v.accepted), cells(v.level)
	if !v.pending {
		solid = target
	}
	lo, hi := min(solid, target), max(solid, target)
	on := lipgloss.NewStyle().Foreground(ColorAccent)
	coming := lipgloss.NewStyle().Foreground(ColorTextDim)
	off := lipgloss.NewStyle().Foreground(ColorBorder)
	label := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	bar := on.Render(strings.Repeat("━", lo)) + coming.Render(strings.Repeat("━", hi-lo)) +
		off.Render(strings.Repeat("─", meterWidth-hi))
	line := on.Render("♪") + "  " + label.Render("Volume") + "  " + bar + "  " +
		label.Render(fmt.Sprintf("%3d%%", v.level))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(v.tint(ColorTextDim)).
		Padding(0, 1).
		Render(line)
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
