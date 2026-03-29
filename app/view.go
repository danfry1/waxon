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
	dimColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.12))

	// Help overlay takes over the whole screen
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
			lipgloss.WithWhitespaceBackground(bg),
			lipgloss.WithWhitespaceForeground(dimColor),
			lipgloss.WithWhitespaceChars(string([]rune(md.PatternChar)[0:1])))
	}

	contentWidth := min(m.width-4, 70)

	var sections []string

	// ── Mood word header ──
	moodName := spacedWord(md.Name)
	dividerWidth := max(0, (contentWidth-len([]rune(moodName))-6)/2)
	dividerL := strings.Repeat("─", dividerWidth)
	dividerR := strings.Repeat("─", dividerWidth)
	moodHeader := fmt.Sprintf("%s  %s  %s", dividerL, moodName, dividerR)
	moodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.5))).
		Align(lipgloss.Center).
		Width(contentWidth)
	sections = append(sections, moodStyle.Render(moodHeader))

	// ── Spacer ──
	sections = append(sections, "")

	// ── Vibe bars (hero element) ──
	visibleBars := min(numBars, contentWidth)
	barHeights := m.bars[:visibleBars]
	barsStr := visual.RenderBars(barHeights, barMaxH, md.Primary, md.Secondary)
	barsStyled := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(contentWidth).
		Render(barsStr)
	sections = append(sections, barsStyled)

	// ── Spacer ──
	sections = append(sections, "")

	// ── Track info ──
	if m.track != nil {
		labelStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.6))).
			Align(lipgloss.Center).
			Width(contentWidth)
		trackStyle := lipgloss.NewStyle().
			Foreground(primary).
			Bold(true).
			Align(lipgloss.Center).
			Width(contentWidth)
		artistStyle := lipgloss.NewStyle().
			Foreground(secondary).
			Align(lipgloss.Center).
			Width(contentWidth)

		sections = append(sections, labelStyle.Render("♫  N O W   P L A Y I N G"))
		sections = append(sections, trackStyle.Render(m.track.Name))
		sections = append(sections, artistStyle.Render(m.track.Artist))
	} else {
		titleStyle := lipgloss.NewStyle().
			Foreground(primary).
			Bold(true).
			Align(lipgloss.Center).
			Width(contentWidth)
		subStyle := lipgloss.NewStyle().
			Foreground(secondary).
			Align(lipgloss.Center).
			Width(contentWidth)

		sections = append(sections, titleStyle.Render("♫  s p o t u i"))
		sections = append(sections, subStyle.Render("waiting for music..."))
		sections = append(sections, subStyle.Render("play something on Spotify to begin"))
	}

	// ── Spacer ──
	sections = append(sections, "")

	// ── Progress bar ──
	if m.track != nil {
		progressStr := m.renderProgress(contentWidth-4, primary, secondary)
		progressStyled := lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(contentWidth).
			Render(progressStr)
		sections = append(sections, progressStyled)

		// Controls
		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controlStr := fmt.Sprintf("⏮      %s      ⏭", playPause)
		controlStyle := lipgloss.NewStyle().
			Foreground(secondary).
			Align(lipgloss.Center).
			Width(contentWidth)
		sections = append(sections, controlStyle.Render(controlStr))
	}

	contentBlock := strings.Join(sections, "\n")

	// Center everything with atmospheric background
	bgChar := string([]rune(md.PatternChar)[0:1])
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, contentBlock,
		lipgloss.WithWhitespaceBackground(bg),
		lipgloss.WithWhitespaceForeground(dimColor),
		lipgloss.WithWhitespaceChars(bgChar))
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

	// Build progress bar with dot indicator
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

func formatDuration(d time.Duration) string {
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
