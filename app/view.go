package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/visual"
)

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

	// Full-width bar area
	barWidth := m.width - 6 // 3 char margin each side
	if barWidth < 20 {
		barWidth = 20
	}

	// Calculate how much vertical space we need
	// mood word: 1, spacer: 2, bars: barMaxH+reflection, spacer: 2,
	// track info: 3-4, spacer: 1, progress: 1, controls: 1
	contentH := 1 + 2 + barMaxH + barMaxH/4 + 2 + 4 + 1 + 1 + 1
	topPad := max(0, (m.height-contentH)/2)

	bgLine := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", m.width))

	var full strings.Builder

	// Top padding — solid dark background
	for range topPad {
		full.WriteString(bgLine)
		full.WriteString("\n")
	}

	// ── Mood word ──
	moodName := spacedWord(md.Name)
	moodColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.4))
	moodStyle := lipgloss.NewStyle().Foreground(moodColor).Background(bg)
	moodLine := centerPad(moodStyle.Render(moodName), m.width, bg)
	full.WriteString(moodLine)
	full.WriteString("\n")

	// Spacer
	full.WriteString(bgLine)
	full.WriteString("\n")
	full.WriteString(bgLine)
	full.WriteString("\n")

	// ── Full-width bars with reflection ──
	visibleBars := min(numBars, barWidth)
	barHeights := m.bars[:visibleBars]
	barsStr := visual.RenderBarsFullWidth(barHeights, barWidth, barMaxH+barMaxH/4, md.Primary, md.Secondary, md.Background, md.Energy, m.pattern)
	for _, line := range strings.Split(barsStr, "\n") {
		centered := centerPad(line, m.width, bg)
		full.WriteString(centered)
		full.WriteString("\n")
	}

	// Spacer
	full.WriteString(bgLine)
	full.WriteString("\n")
	full.WriteString(bgLine)
	full.WriteString("\n")

	// ── Track info with album art ──
	if m.track != nil {
		labelColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.45))
		labelStyle := lipgloss.NewStyle().Foreground(labelColor).Background(bg)
		trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		// Try to render album art
		artSize := 12 // character rows (24 pixels tall)
		artStr := m.artwork.RenderArtwork(m.track.ArtworkURL, artSize*2, artSize)

		if artStr != "" {
			// Layout: art on left, track info on right
			artBlock := lipgloss.NewStyle().Background(bg).Render(artStr)

			infoLines := strings.Join([]string{
				"",
				"",
				labelStyle.Render("♫  N O W   P L A Y I N G"),
				"",
				trackStyle.Render(m.track.Name),
				artistStyle.Render(m.track.Artist),
			}, "\n")
			infoBlock := lipgloss.NewStyle().
				Background(bg).
				PaddingLeft(3).
				Width(40).
				Render(infoLines)

			combined := lipgloss.JoinHorizontal(lipgloss.Top, artBlock, infoBlock)
			full.WriteString(centerPad(combined, m.width, bg))
			full.WriteString("\n")
		} else {
			full.WriteString(centerPad(labelStyle.Render("♫  N O W   P L A Y I N G"), m.width, bg))
			full.WriteString("\n")
			full.WriteString(centerPad(trackStyle.Render(m.track.Name), m.width, bg))
			full.WriteString("\n")
			full.WriteString(centerPad(artistStyle.Render(m.track.Artist), m.width, bg))
			full.WriteString("\n")
		}
	} else {
		titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		subStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		full.WriteString(centerPad(titleStyle.Render("♫  s p o t u i"), m.width, bg))
		full.WriteString("\n")
		full.WriteString(centerPad(subStyle.Render("waiting for music..."), m.width, bg))
		full.WriteString("\n")
		full.WriteString(centerPad(subStyle.Render("play something on Spotify to begin"), m.width, bg))
		full.WriteString("\n")
	}

	// Spacer
	full.WriteString(bgLine)
	full.WriteString("\n")

	// ── Progress bar ──
	if m.track != nil {
		progressWidth := min(m.width-20, 50)
		progressStr := m.renderProgress(progressWidth, primary, secondary)
		full.WriteString(centerPad(progressStr, m.width, bg))
		full.WriteString("\n")

		// Controls
		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controlStr := fmt.Sprintf("⏮      %s      ⏭", playPause)
		controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		full.WriteString(centerPad(controlStyle.Render(controlStr), m.width, bg))
		full.WriteString("\n")
	}

	// Fill remaining with solid background
	currentLines := strings.Count(full.String(), "\n")
	remaining := m.height - currentLines
	for range max(0, remaining) {
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

// centerPad centers rendered text within the given width, filling sides with bg color.
func centerPad(rendered string, totalWidth int, bg lipgloss.Color) string {
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
