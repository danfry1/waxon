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

// contentLine represents one line of content to render.
// Either 'raw' (pre-styled ANSI string, e.g. album art) or 'text' (plain text to be styled by compositor).
type contentLine struct {
	raw  string // pre-styled string — used as-is (album art)
	text string // plain text — compositor applies fg + glow bg
	fg   string // foreground color for text mode
	bold bool   // bold for text mode
}

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

	// Panel overlay
	if m.activePanel != PanelNone && m.panel != nil {
		panelView := m.panel.View(md.Primary, md.Secondary, md.Background)
		return lipgloss.Place(m.width, m.height, lipgloss.Right, lipgloss.Center, panelView,
			lipgloss.WithWhitespaceBackground(bg))
	}

	// Get particle grid and glow grid
	var particleGrid [][]rune
	if m.effects.Particles != nil {
		particleGrid = m.effects.Particles.Render()
	}
	glowGrid := m.effects.GlowGrid

	// Build content
	content := m.buildContentLines(md)

	// Bar configuration — bars at the very bottom
	barH := max(5, min(8, m.height/5))
	smoothBars := visual.InterpolateBars(m.bars[:], m.width)

	// Layout: content centered above bars
	contentAreaH := m.height - barH
	contentH := len(content)
	topPad := max(1, (contentAreaH-contentH)/2)

	// Particle color
	particleFg := visual.LerpColor(md.Background, md.Primary, 0.55)

	// Render
	var full strings.Builder
	full.Grow(m.width * m.height * 5)

	for row := range m.height {
		if row > 0 {
			full.WriteByte('\n')
		}

		barRow := row - (m.height - barH)

		if barRow >= 0 {
			// Bar row
			m.renderBarRow(&full, row, barRow, barH, smoothBars, glowGrid, particleGrid, md, particleFg)
		} else {
			ci := row - topPad
			if ci >= 0 && ci < contentH {
				m.renderContentLine(&full, row, content[ci], glowGrid, particleGrid, md, particleFg)
			} else {
				m.renderGlowRow(&full, row, 0, m.width, glowGrid, particleGrid, md, particleFg)
			}
		}
	}

	return full.String()
}

// glowBgAt returns the glow background color at a screen position.
func glowBgAt(row, col int, glowGrid [][]string, fallback string) string {
	if row < len(glowGrid) && col < len(glowGrid[row]) && glowGrid[row][col] != "" {
		return glowGrid[row][col]
	}
	return fallback
}

// renderContentLine renders one content line, compositing text over glow backgrounds.
func (m Model) renderContentLine(sb *strings.Builder, row int, cl contentLine, glowGrid [][]string, particleGrid [][]rune, md mood.Mood, particleFg string) {
	if cl.raw != "" {
		// Pre-styled line (album art) — center it, fill sides with glow
		contentW := lipgloss.Width(cl.raw)
		if contentW >= m.width {
			sb.WriteString(cl.raw)
			return
		}
		leftPad := (m.width - contentW) / 2
		rightPad := m.width - contentW - leftPad
		m.renderGlowRow(sb, row, 0, leftPad, glowGrid, particleGrid, md, particleFg)
		sb.WriteString(cl.raw)
		m.renderGlowRow(sb, row, leftPad+contentW, rightPad, glowGrid, particleGrid, md, particleFg)
		return
	}

	if cl.text == "" {
		// Blank line — full glow
		m.renderGlowRow(sb, row, 0, m.width, glowGrid, particleGrid, md, particleFg)
		return
	}

	// Text line — render each character with glow background
	textRunes := []rune(cl.text)
	textW := len(textRunes)
	leftPad := (m.width - textW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	rightPad := m.width - textW - leftPad
	if rightPad < 0 {
		rightPad = 0
	}

	// Left glow
	m.renderGlowRow(sb, row, 0, leftPad, glowGrid, particleGrid, md, particleFg)

	// Text characters with glow backgrounds — batch by same bg color
	i := 0
	for i < textW {
		col := leftPad + i
		cellBg := glowBgAt(row, col, glowGrid, md.Background)

		// Find run of characters with same background
		j := i + 1
		for j < textW {
			nextCol := leftPad + j
			nextBg := glowBgAt(row, nextCol, glowGrid, md.Background)
			if nextBg != cellBg {
				break
			}
			j++
		}

		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cl.fg)).
			Background(lipgloss.Color(cellBg))
		if cl.bold {
			style = style.Bold(true)
		}
		sb.WriteString(style.Render(string(textRunes[i:j])))
		i = j
	}

	// Right glow
	m.renderGlowRow(sb, row, leftPad+textW, rightPad, glowGrid, particleGrid, md, particleFg)
}

// renderGlowRow renders cells with glow background and particles.
func (m Model) renderGlowRow(sb *strings.Builder, row, startCol, count int, glowGrid [][]string, particleGrid [][]rune, md mood.Mood, particleFg string) {
	if count <= 0 {
		return
	}

	hasParticles := row < len(particleGrid) && len(particleGrid[row]) > 0

	i := 0
	for i < count {
		col := startCol + i
		cellBg := glowBgAt(row, col, glowGrid, md.Background)

		// Check particle
		if hasParticles && col < len(particleGrid[row]) && particleGrid[row][col] != 0 {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(particleFg)).
				Background(lipgloss.Color(cellBg))
			sb.WriteString(style.Render(string(particleGrid[row][col])))
			i++
			continue
		}

		// Batch consecutive space cells with same bg
		j := i + 1
		for j < count {
			nextCol := startCol + j
			nextBg := glowBgAt(row, nextCol, glowGrid, md.Background)
			// Break if different bg or has particle
			if nextBg != cellBg {
				break
			}
			if hasParticles && nextCol < len(particleGrid[row]) && particleGrid[row][nextCol] != 0 {
				break
			}
			j++
		}

		style := lipgloss.NewStyle().Background(lipgloss.Color(cellBg))
		sb.WriteString(style.Render(strings.Repeat(" ", j-i)))
		i = j
	}
}

// renderBarRow renders bars with per-column glow backgrounds and color gradient.
func (m Model) renderBarRow(sb *strings.Builder, screenRow, barRow, barH int, smoothBars []float64, glowGrid [][]string, particleGrid [][]rune, md mood.Mood, particleFg string) {
	hasParticles := screenRow < len(particleGrid) && len(particleGrid[screenRow]) > 0

	// Vertical gradient — bottom rows are secondary, top rows are primary
	rowRatio := 1.0 - float64(barRow)/float64(barH)

	i := 0
	for i < m.width {
		col := i
		cellBg := glowBgAt(screenRow, col, glowGrid, md.Background)

		if col < len(smoothBars) {
			h := smoothBars[col]
			barPx := h * float64(barH)
			rowFromBottom := barH - 1 - barRow

			if float64(rowFromBottom) < barPx-1 {
				// Full bar cell — add horizontal gradient too (left-to-right color shift)
				horizT := float64(col) / float64(m.width)
				barFg := visual.LerpColor(
					visual.LerpColor(md.Secondary, md.Primary, 0.2+rowRatio*0.8),
					visual.LerpColor(md.Primary, md.Secondary, 0.3),
					horizT*0.3,
				)
				// Tint the background slightly with bar color for glow effect
				barBg := visual.LerpColor(cellBg, barFg, 0.08)

				// Batch consecutive filled cells
				j := i + 1
				for j < m.width && j < len(smoothBars) {
					nh := smoothBars[j]
					nBarPx := nh * float64(barH)
					if float64(rowFromBottom) >= nBarPx-1 {
						break
					}
					j++
				}

				style := lipgloss.NewStyle().
					Foreground(lipgloss.Color(barFg)).
					Background(lipgloss.Color(barBg))
				sb.WriteString(style.Render(strings.Repeat("█", j-i)))
				i = j
				continue
			} else if float64(rowFromBottom) < barPx {
				// Partial bar cell
				frac := barPx - float64(int(barPx))
				idx := int(frac * float64(len(visual.BarChars)-1))
				idx = max(0, min(idx, len(visual.BarChars)-1))
				barFg := visual.LerpColor(md.Secondary, md.Primary, 0.2+rowRatio*0.8)
				style := lipgloss.NewStyle().
					Foreground(lipgloss.Color(barFg)).
					Background(lipgloss.Color(cellBg))
				sb.WriteString(style.Render(visual.BarChars[idx]))
				i++
				continue
			}
		}

		// Empty cell in bar area — particle or space
		if hasParticles && col < len(particleGrid[screenRow]) && particleGrid[screenRow][col] != 0 {
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(particleFg)).
				Background(lipgloss.Color(cellBg))
			sb.WriteString(style.Render(string(particleGrid[screenRow][col])))
			i++
			continue
		}

		// Batch empty cells
		j := i + 1
		for j < m.width {
			nextBg := glowBgAt(screenRow, j, glowGrid, md.Background)
			if nextBg != cellBg {
				break
			}
			if j < len(smoothBars) {
				nh := smoothBars[j]
				nBarPx := nh * float64(barH)
				rowFromBottom := barH - 1 - barRow
				if float64(rowFromBottom) < nBarPx {
					break
				}
			}
			if hasParticles && j < len(particleGrid[screenRow]) && particleGrid[screenRow][j] != 0 {
				break
			}
			j++
		}

		style := lipgloss.NewStyle().Background(lipgloss.Color(cellBg))
		sb.WriteString(style.Render(strings.Repeat(" ", j-i)))
		i = j
	}
}

// buildContentLines returns content as structured lines.
// Text lines have fg/bold info — the compositor applies glow backgrounds per-cell.
// Raw lines (album art) are pre-styled ANSI strings.
func (m Model) buildContentLines(md mood.Mood) []contentLine {
	var lines []contentLine

	// Album art
	if m.artworkRendered != "" {
		if m.artworkIsKitty {
			lines = append(lines, contentLine{raw: m.artworkRendered})
			for range m.artworkRows {
				lines = append(lines, contentLine{})
			}
		} else {
			for _, line := range strings.Split(m.artworkRendered, "\n") {
				lines = append(lines, contentLine{raw: line})
			}
		}
		lines = append(lines, contentLine{}) // spacer
	}

	// Track info
	if m.track != nil {
		labelFg := visual.LerpColor(md.Background, md.Primary, 0.45)
		lines = append(lines, contentLine{text: "N O W   P L A Y I N G", fg: labelFg})
		lines = append(lines, contentLine{text: m.track.Name, fg: md.Primary, bold: true})
		lines = append(lines, contentLine{text: m.track.Artist, fg: md.Secondary})
	} else {
		lines = append(lines, contentLine{text: "♫  s p o t u i", fg: md.Primary, bold: true})
		lines = append(lines, contentLine{text: "waiting for music...", fg: md.Secondary})
	}

	lines = append(lines, contentLine{}) // spacer

	// Progress + controls
	if m.track != nil {
		// Progress bar as raw (it has complex styling)
		progressWidth := min(m.width-20, 50)
		progressStr := m.renderProgress(progressWidth, md)
		lines = append(lines, contentLine{raw: progressStr})

		// Controls
		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controlStr := fmt.Sprintf("⏮      %s      ⏭", playPause)
		lines = append(lines, contentLine{text: controlStr, fg: md.Secondary})

		// Status line
		var statusParts []string
		if m.richSource != nil {
			statusParts = append(statusParts, fmt.Sprintf("♪ %d%%", m.volume))
			if m.shuffleOn {
				statusParts = append(statusParts, "⤮ on")
			}
			switch m.repeatMode {
			case source.RepeatContext:
				statusParts = append(statusParts, "↻ all")
			case source.RepeatTrack:
				statusParts = append(statusParts, "↻ one")
			}
		}
		if len(statusParts) > 0 {
			dimFg := visual.LerpColor(md.Background, md.Primary, 0.3)
			lines = append(lines, contentLine{text: strings.Join(statusParts, "  ·  "), fg: dimFg})
		}
	}

	return lines
}

// renderProgress renders the progress bar (returned as raw styled string).
func (m Model) renderProgress(width int, md mood.Mood) string {
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

	primary := lipgloss.Color(md.Primary)
	secondary := lipgloss.Color(md.Secondary)

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
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
