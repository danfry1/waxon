package visual

import (
	"math"
)

// RenderGlow creates a full-screen glow grid emanating from the art center.
// Every cell gets a color — cells closer to the art are brighter, edges fade gently.
func RenderGlow(screenW, screenH, artW, artH int, primary, secondary, background string, energy float64) [][]string {
	grid := make([][]string, screenH)
	for i := range grid {
		grid[i] = make([]string, screenW)
	}

	cx := float64(screenW) / 2
	cy := float64(screenH) / 2

	artHalfW := float64(artW) / 2
	artHalfH := float64(artH) / 2
	if artHalfW < 1 {
		artHalfW = 1
	}
	if artHalfH < 1 {
		artHalfH = 1
	}

	intensity := 0.25 + energy*0.35

	// Maximum distance from center to corner for normalization
	maxDist := math.Sqrt(float64(screenW*screenW)/4 + float64(screenH*screenH)/4)
	if maxDist < 1 {
		maxDist = 1
	}

	for row := range screenH {
		for col := range screenW {
			// Skip cells inside the art area
			if math.Abs(float64(col)-cx) <= artHalfW &&
				math.Abs(float64(row)-cy) <= artHalfH {
				continue
			}

			// Normalized distance from art edge
			dx := (float64(col) - cx) / artHalfW
			dy := (float64(row) - cy) / artHalfH
			dist := math.Sqrt(dx*dx + dy*dy)

			normalizedDist := dist - 1.0
			if normalizedDist < 0 {
				normalizedDist = 0
			}

			// Gentler falloff so glow reaches edges
			falloff := math.Exp(-normalizedDist * 0.25)

			// Even at edges, maintain a minimum glow tint
			if falloff < 0.03 {
				falloff = 0.03
			}

			colorT := math.Min(normalizedDist/4.0, 1.0)
			glowColor := LerpColor(primary, secondary, colorT)

			t := falloff * intensity
			color := LerpColor(background, glowColor, t)
			grid[row][col] = color
		}
	}

	return grid
}
