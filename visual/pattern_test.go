package visual

import (
	"strings"
	"testing"
)

func TestRenderPattern(t *testing.T) {
	row := RenderPatternRow("~ · ", 20)
	if len([]rune(row)) != 20 { t.Errorf("row rune length = %d, want 20", len([]rune(row))) }
	if !strings.Contains(row, "~") { t.Error("expected row to contain pattern characters") }
}

func TestRenderPatternRows(t *testing.T) {
	rows := RenderPatternRows("╱╲", 30, 5, 0)
	if len(rows) != 5 { t.Errorf("got %d rows, want 5", len(rows)) }
	for i, row := range rows {
		if len([]rune(row)) != 30 { t.Errorf("row %d rune length = %d, want 30", i, len([]rune(row))) }
	}
}

func TestRenderPatternRowsOffset(t *testing.T) {
	rows0 := RenderPatternRows("~ · ", 20, 1, 0)
	rows1 := RenderPatternRows("~ · ", 20, 1, 1)
	if rows0[0] == rows1[0] { t.Error("expected different rows with different offsets") }
}
