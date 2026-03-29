package visual

import (
	"strings"
	"testing"
)

func TestRenderBars(t *testing.T) {
	heights := []float64{0.5, 0.8, 0.3, 1.0, 0.0}
	result := RenderBars(heights, 6, "#ff0000", "#880000")
	lines := strings.Split(result, "\n")
	if len(lines) != 6 { t.Errorf("got %d lines, want 6", len(lines)) }
}

func TestRenderBarsEmpty(t *testing.T) {
	result := RenderBars(nil, 4, "#ff0000", "#880000")
	if result == "" { t.Error("expected non-empty output even with no bars") }
}
