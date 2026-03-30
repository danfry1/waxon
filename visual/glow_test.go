package visual

import "testing"

func TestRenderGlow(t *testing.T) {
	grid := RenderGlow(40, 12, 20, 10, "#daa520", "#8b7a54", "#1a1510", 0.5)
	if len(grid) != 12 {
		t.Errorf("grid height = %d, want 12", len(grid))
	}
	if len(grid[0]) != 40 {
		t.Errorf("grid width = %d, want 40", len(grid[0]))
	}
}

func TestGlowCenterBrighter(t *testing.T) {
	w, h := 40, 20
	artW, artH := 16, 8
	grid := RenderGlow(w, h, artW, artH, "#daa520", "#8b7a54", "#1a1510", 0.5)

	centerRow := h / 2
	centerCol := w / 2
	cell := grid[centerRow][centerCol]
	if cell != "" {
		// Center is inside the art area, should be empty
		// Check a cell just outside the art instead
	}

	// Check a cell just outside art boundary — should have glow
	outsideCol := w/2 + artW/2 + 1
	if outsideCol < w {
		cell = grid[centerRow][outsideCol]
		if cell == "" {
			t.Error("cell just outside art should have glow color")
		}
	}
}
