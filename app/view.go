package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/visual"
)

const bottomBarH = 6

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "starting..."
	}

	md := m.mood
	primary := lipgloss.Color(md.Primary)
	secondary := lipgloss.Color(md.Secondary)
	bg := lipgloss.Color(md.Background)

	bgLine := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", m.width))

	// Help overlay
	if m.showHelp {
		helpStr := m.help.View(m.keys)
		helpStyled := lipgloss.NewStyle().
			Foreground(secondary).
			Background(bg).
			Padding(2, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.3))).
			Render(helpStr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpStyled,
			lipgloss.WithWhitespaceBackground(bg))
	}

	// Build content sections
	var sections []string

	// ── Mood word ──
	moodColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.35))
	moodStyle := lipgloss.NewStyle().Foreground(moodColor).Background(bg)
	sections = append(sections, centerLine(moodStyle.Render(spacedWord(md.Name)), m.width, bg))
	sections = append(sections, bgLine)

	// ── Album art (hero element) ──
	if m.artworkRendered != "" {
		artLines := strings.Split(m.artworkRendered, "\n")
		for _, line := range artLines {
			sections = append(sections, centerLine(line, m.width, bg))
		}
		sections = append(sections, bgLine)
	}

	// ── Track info ──
	if m.track != nil {
		labelColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.4))
		labelStyle := lipgloss.NewStyle().Foreground(labelColor).Background(bg)
		trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		sections = append(sections, centerLine(labelStyle.Render("N O W   P L A Y I N G"), m.width, bg))
		sections = append(sections, centerLine(trackStyle.Render(m.track.Name), m.width, bg))
		sections = append(sections, centerLine(artistStyle.Render(m.track.Artist), m.width, bg))
	} else {
		titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		subStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		sections = append(sections, centerLine(titleStyle.Render("♫  s p o t u i"), m.width, bg))
		sections = append(sections, centerLine(subStyle.Render("waiting for music..."), m.width, bg))
	}

	sections = append(sections, bgLine)

	// ── Progress bar ──
	if m.track != nil {
		progressWidth := min(m.width-20, 50)
		progressStr := m.renderProgress(progressWidth, primary, secondary)
		sections = append(sections, centerLine(progressStr, m.width, bg))

		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controlStr := fmt.Sprintf("⏮      %s      ⏭", playPause)
		controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		sections = append(sections, centerLine(controlStyle.Render(controlStr), m.width, bg))
	}

	// Calculate vertical centering
	contentH := len(sections)
	barsH := bottomBarH + bottomBarH/4 // bars + reflection
	totalContentH := contentH + 1 + barsH // +1 for spacer before bars

	topPad := max(0, (m.height-totalContentH)/2)

	var full strings.Builder

	// Top padding
	for range topPad {
		full.WriteString(bgLine)
		full.WriteString("\n")
	}

	// Main content
	for _, line := range sections {
		full.WriteString(line)
		full.WriteString("\n")
	}

	// Spacer before bars
	full.WriteString(bgLine)
	full.WriteString("\n")

	// ── Bottom accent bars ──
	barWidth := m.width - 2
	visibleBars := min(numBars, barWidth)
	barHeights := m.bars[:visibleBars]
	barsStr := visual.RenderBarsFullWidth(barHeights, barWidth, barsH, md.Primary, md.Secondary, md.Background, md.Energy, m.pattern)
	for _, line := range strings.Split(barsStr, "\n") {
		full.WriteString(centerLine(line, m.width, bg))
		full.WriteString("\n")
	}

	// Bottom padding
	currentLines := topPad + totalContentH
	for range max(0, m.height-currentLines) {
		full.WriteString(bgLine)
		full.WriteString("\n")
	}

	return full.String()
}

func (m Model) renderProgress(width int, primary, secondary lipgloss.Color) string {
	if m.track == nil {
		return ""
	}
	pos := m.track.Position
	dur := m.track.Duration
	if dur == 0 {
		return ""
	}

	barWidth := width - 14
	if barWidth < 10 {
		barWidth = 10
	}
	ratio := float64(pos) / float64(dur)
	filled := int(float64(barWidth) * ratio)
	filled = max(0, min(filled, barWidth-1))

	var bar strings.Builder
	filledStyle := lipgloss.NewStyle().Foreground(primary)
	dotStyle := lipgloss.NewStyle().Foreground(primary).Bold(true)
	emptyStyle := lipgloss.NewStyle().Foreground(secondary)

	bar.WriteString(filledStyle.Render(strings.Repeat("━", filled)))
	bar.WriteString(dotStyle.Render("●"))
	remaining := barWidth - filled - 1
	if remaining > 0 {
		bar.WriteString(emptyStyle.Render(strings.Repeat("━", remaining)))
	}

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)
	timeStyle := lipgloss.NewStyle().Foreground(secondary)

	return fmt.Sprintf("%s %s", bar.String(), timeStyle.Render(fmt.Sprintf("%s / %s", posStr, durStr)))
}

// centerLine centers rendered text within the given width, filling sides with bg.
func centerLine(rendered string, totalWidth int, bg lipgloss.Color) string {
	w := lipgloss.Width(rendered)
	if w >= totalWidth {
		return rendered
	}
	leftPad := (totalWidth - w) / 2
	rightPad := totalWidth - w - leftPad
	bgStyle := lipgloss.NewStyle().Background(bg)
	return bgStyle.Render(strings.Repeat(" ", leftPad)) + rendered + bgStyle.Render(strings.Repeat(" ", rightPad))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func spacedWord(s string) string {
	runes := []rune(s)
	spaced := make([]rune, 0, len(runes)*2-1)
	for i, r := range runes {
		if i > 0 {
			spaced = append(spaced, ' ')
		}
		spaced = append(spaced, r)
	}
	return string(spaced)
}
