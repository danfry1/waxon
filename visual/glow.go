package visual

import (
	"math"
)

func RenderGlow(screenW, screenH, artW, artH int, primary, secondary, background string, energy float64) [][]string {
	grid := make([][]string, screenH)
	for i := range grid {
		grid[i] = make([]string, screenW)
	}

	cx := float64(screenW) / 2
	cy := float64(screenH) / 2

	maxRadius := math.Max(float64(artW), float64(artH)) * (1.5 + energy*1.5)
	if maxRadius < 8 {
		maxRadius = 8
	}

	intensity := 0.15 + energy*0.25

	for row := range screenH {
		for col := range screenW {
			dx := (float64(col) - cx) / (float64(artW) / 2)
			dy := (float64(row) - cy) / (float64(artH) / 2)
			dist := math.Sqrt(dx*dx + dy*dy)

			if math.Abs(float64(col)-cx) <= float64(artW)/2 &&
				math.Abs(float64(row)-cy) <= float64(artH)/2 {
				continue
			}

			normalizedDist := dist - 1.0
			if normalizedDist < 0 {
				normalizedDist = 0
			}

			falloff := math.Exp(-normalizedDist * 0.8)
			if falloff < 0.02 {
				continue
			}

			colorT := math.Min(normalizedDist/3.0, 1.0)
			glowColor := LerpColor(primary, secondary, colorT)

			t := falloff * intensity
			color := LerpColor(background, glowColor, t)
			grid[row][col] = color
		}
	}

	return grid
}
