package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
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
	bg := lipgloss.Color(md.Background)

	// Help overlay
	if m.showHelp {
		secondary := lipgloss.Color(md.Secondary)
		helpStr := m.help.View(m.keys)
		helpStyled := lipgloss.NewStyle().
			Foreground(secondary).Background(bg).
			Padding(2, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.3))).
			Render(helpStr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpStyled,
			lipgloss.WithWhitespaceBackground(bg))
	}

	// Panel overlay — render over background
	if m.activePanel != PanelNone && m.panel != nil {
		panelView := m.panel.View(md.Primary, md.Secondary, md.Background)
		return lipgloss.Place(m.width, m.height, lipgloss.Right, lipgloss.Center, panelView,
			lipgloss.WithWhitespaceBackground(bg))
	}

	// Get particle grid
	var particleGrid [][]rune
	if m.effects.Particles != nil {
		particleGrid = m.effects.Particles.Render()
	}

	glowGrid := m.effects.GlowGrid

	// Build content lines (no bars — bars are rendered separately at the bottom)
	contentLines := m.buildContentLines(md)

	// Bar configuration
	barH := max(6, min(10, m.height/4))
	smoothBars := visual.InterpolateBars(m.bars[:], m.width)

	// Layout: content centered vertically in the space above the bars
	contentAreaH := m.height - barH
	contentH := len(contentLines)
	topPad := max(0, (contentAreaH-contentH)/2)

	// Pre-compute a base background color from glow intensity
	var glowIntensity float64
	if m.track != nil && m.track.Playing {
		glowIntensity = 0.20
	} else {
		glowIntensity = 0.06
	}
	glowIntensity += m.effects.Breathing
	_ = glowIntensity // used via glow grid

	// Render row by row
	var full strings.Builder
	full.Grow(m.width * m.height * 4) // rough estimate

	for row := range m.height {
		if row > 0 {
			full.WriteByte('\n')
		}

		barRow := row - (m.height - barH)

		if barRow >= 0 {
			// --- Bar row ---
			m.renderBarRow(&full, row, barRow, barH, smoothBars, glowGrid, particleGrid, md)
		} else {
			// --- Content or empty row ---
			ci := row - topPad
			if ci >= 0 && ci < contentH {
				// Content row: center content, fill sides with glow+particles
				m.renderContentRow(&full, row, contentLines[ci], glowGrid, particleGrid, md)
			} else {
				// Empty row: full glow bg + particles
				m.renderEmptyRow(&full, row, glowGrid, particleGrid, md)
			}
		}
	}

	return full.String()
}

// renderEmptyRow renders a full-width row of glow background and particles.
func (m Model) renderEmptyRow(sb *strings.Builder, row int, glowGrid [][]string, particleGrid [][]rune, md mood.Mood) {
	m.renderGlowRow(sb, row, 0, m.width, glowGrid, particleGrid, md)
}

// renderContentRow centers content text on the row and fills sides with glow.
func (m Model) renderContentRow(sb *strings.Builder, row int, content string, glowGrid [][]string, particleGrid [][]rune, md mood.Mood) {
	contentW := lipgloss.Width(content)
	if contentW >= m.width {
		sb.WriteString(content)
		return
	}
	leftPad := (m.width - contentW) / 2
	rightPad := m.width - contentW - leftPad

	// Left side glow
	m.renderGlowRow(sb, row, 0, leftPad, glowGrid, particleGrid, md)
	// Content (already has its own styling)
	sb.WriteString(content)
	// Right side glow
	m.renderGlowRow(sb, row, leftPad+contentW, rightPad, glowGrid, particleGrid, md)
}

// renderGlowRow renders `count` cells starting at `startCol` using glow grid and particles.
// Batches adjacent cells with the same background color for performance.
func (m Model) renderGlowRow(sb *strings.Builder, row, startCol, count int, glowGrid [][]string, particleGrid [][]rune, md mood.Mood) {
	if count <= 0 {
		return
	}

	hasGlow := len(glowGrid) > row && len(glowGrid[row]) > 0
	hasParticles := len(particleGrid) > row && len(particleGrid[row]) > 0

	type cell struct {
		bg   string
		ch   rune // 0 means space
		pFg  string
	}

	// Build cell data
	cells := make([]cell, count)
	for i := range count {
		col := startCol + i
		c := cell{bg: md.Background}

		if hasGlow && col < len(glowGrid[row]) && glowGrid[row][col] != "" {
			c.bg = glowGrid[row][col]
		}

		if hasParticles && col < len(particleGrid[row]) && particleGrid[row][col] != 0 {
			c.ch = particleGrid[row][col]
			c.pFg = visual.LerpColor(md.Background, md.Primary, 0.6)
		}

		cells[i] = c
	}

	// Batch render: group adjacent cells with same bg and no particle content into one string
	i := 0
	for i < len(cells) {
		c := cells[i]

		if c.ch != 0 {
			// Particle cell — render individually
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(c.pFg)).
				Background(lipgloss.Color(c.bg))
			sb.WriteString(style.Render(string(c.ch)))
			i++
			continue
		}

		// Batch plain space cells with same bg
		j := i + 1
		for j < len(cells) && cells[j].ch == 0 && cells[j].bg == c.bg {
			j++
		}
		n := j - i
		style := lipgloss.NewStyle().Background(lipgloss.Color(c.bg))
		sb.WriteString(style.Render(strings.Repeat(" ", n)))
		i = j
	}
}

// renderBarRow renders one row of the bar visualizer with glow-tinted backgrounds.
func (m Model) renderBarRow(sb *strings.Builder, screenRow, barRow, barH int, smoothBars []float64, glowGrid [][]string, particleGrid [][]rune, md mood.Mood) {
	hasGlow := len(glowGrid) > screenRow && len(glowGrid[screenRow]) > 0
	hasParticles := len(particleGrid) > screenRow && len(particleGrid[screenRow]) > 0

	// Pre-compute row gradient color for bar foreground
	rowRatio := 1.0 - float64(barRow)/float64(barH)
	barFgColor := visual.LerpColor(md.Secondary, md.Primary, 0.2+rowRatio*0.8)

	// For each column, determine: is it a bar cell or empty?
	type barCell struct {
		isFilled bool
		isPartial bool
		partialIdx int
		bg string
		particle rune
		particleFg string
	}

	cells := make([]barCell, m.width)
	for col := range m.width {
		bc := barCell{bg: md.Background}

		// Glow background
		if hasGlow && col < len(glowGrid[screenRow]) && glowGrid[screenRow][col] != "" {
			bc.bg = glowGrid[screenRow][col]
		}

		// Bar height check
		if col < len(smoothBars) {
			h := smoothBars[col]
			barPx := h * float64(barH)
			rowFromBottom := barH - 1 - barRow

			if float64(rowFromBottom) < barPx-1 {
				bc.isFilled = true
			} else if float64(rowFromBottom) < barPx {
				bc.isPartial = true
				frac := barPx - float64(int(barPx))
				idx := int(frac * float64(len(visual.BarChars)-1))
				if idx < 0 {
					idx = 0
				}
				if idx >= len(visual.BarChars) {
					idx = len(visual.BarChars) - 1
				}
				bc.partialIdx = idx
			} else {
				// Empty bar cell — check for particle
				if hasParticles && col < len(particleGrid[screenRow]) && particleGrid[screenRow][col] != 0 {
					bc.particle = particleGrid[screenRow][col]
					bc.particleFg = visual.LerpColor(md.Background, md.Primary, 0.6)
				}
			}
		}
		cells[col] = bc
	}

	// Render with batching: group adjacent cells of the same type
	i := 0
	for i < len(cells) {
		c := cells[i]

		if c.isFilled {
			// Batch consecutive filled cells with same bg
			j := i + 1
			for j < len(cells) && cells[j].isFilled && cells[j].bg == c.bg {
				j++
			}
			n := j - i
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(barFgColor)).
				Background(lipgloss.Color(c.bg))
			sb.WriteString(style.Render(strings.Repeat("\u2588", n)))
			i = j
			continue
		}

		if c.isPartial {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(barFgColor)).
				Background(lipgloss.Color(c.bg))
			sb.WriteString(style.Render(visual.BarChars[c.partialIdx]))
			i++
			continue
		}

		if c.particle != 0 {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(c.particleFg)).
				Background(lipgloss.Color(c.bg))
			sb.WriteString(style.Render(string(c.particle)))
			i++
			continue
		}

		// Empty cell — batch consecutive empty with same bg
		j := i + 1
		for j < len(cells) && !cells[j].isFilled && !cells[j].isPartial && cells[j].particle == 0 && cells[j].bg == c.bg {
			j++
		}
		n := j - i
		style := lipgloss.NewStyle().Background(lipgloss.Color(c.bg))
		sb.WriteString(style.Render(strings.Repeat(" ", n)))
		i = j
	}
}

// buildContentLines produces the styled content lines (mood label, art, track info, progress, controls).
// Lines are styled but NOT padded to full width — the View compositor handles side fill.
func (m Model) buildContentLines(md mood.Mood) []string {
	primary := lipgloss.Color(md.Primary)
	secondary := lipgloss.Color(md.Secondary)
	bg := lipgloss.Color(md.Background)

	var lines []string

	// Mood word
	moodColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.35))
	moodStyle := lipgloss.NewStyle().Foreground(moodColor).Background(bg)
	lines = append(lines, moodStyle.Render(spacedWord(md.Name)))

	// Blank line
	lines = append(lines, "")

	// Album art
	if m.artworkRendered != "" {
		if m.artworkIsKitty {
			lines = append(lines, m.artworkRendered)
			for range m.artworkRows {
				lines = append(lines, "")
			}
		} else {
			for _, line := range strings.Split(m.artworkRendered, "\n") {
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")
	}

	// Track info
	if m.track != nil {
		labelColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.4))
		labelStyle := lipgloss.NewStyle().Foreground(labelColor).Background(bg)
		trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)

		lines = append(lines, labelStyle.Render("N O W   P L A Y I N G"))
		lines = append(lines, trackStyle.Render(m.track.Name))
		lines = append(lines, artistStyle.Render(m.track.Artist))
	} else {
		titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
		subStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		lines = append(lines, titleStyle.Render("\u266b  s p o t u i"))
		lines = append(lines, subStyle.Render("waiting for music..."))
	}

	lines = append(lines, "")

	// Progress + controls
	if m.track != nil {
		progressWidth := min(m.width-20, 50)
		progressStr := m.renderProgress(progressWidth, primary, secondary)
		lines = append(lines, progressStr)

		playPause := "\u25b6"
		if m.track.Playing {
			playPause = "\u23f8"
		}
		controlStr := fmt.Sprintf("\u23ee      %s      \u23ed", playPause)
		controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
		lines = append(lines, controlStyle.Render(controlStr))

		// Status line: volume, shuffle, repeat
		var statusParts []string
		if m.richSource != nil {
			statusParts = append(statusParts, fmt.Sprintf("\u266a %d%%", m.volume))
			if m.shuffleOn {
				statusParts = append(statusParts, "\u292e on")
			}
			switch m.repeatMode {
			case source.RepeatContext:
				statusParts = append(statusParts, "\u21bb all")
			case source.RepeatTrack:
				statusParts = append(statusParts, "\u21bb one")
			}
		}
		if len(statusParts) > 0 {
			dimColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.25))
			dimStyle := lipgloss.NewStyle().Foreground(dimColor).Background(bg)
			lines = append(lines, dimStyle.Render(strings.Join(statusParts, "  \u00b7  ")))
		}
	}

	return lines
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

	bar.WriteString(filledStyle.Render(strings.Repeat("\u2501", filled)))
	bar.WriteString(dotStyle.Render("\u25cf"))
	remaining := barWidth - filled - 1
	if remaining > 0 {
		bar.WriteString(emptyStyle.Render(strings.Repeat("\u2501", remaining)))
	}

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)
	timeStyle := lipgloss.NewStyle().Foreground(secondary)
	return fmt.Sprintf("%s %s", bar.String(), timeStyle.Render(fmt.Sprintf("%s / %s", posStr, durStr)))
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
