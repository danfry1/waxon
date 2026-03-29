package visual

import "testing"

func TestHexToRGB(t *testing.T) {
	r, g, b, err := HexToRGB("#ff8800")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if r != 255 || g != 136 || b != 0 { t.Errorf("got (%d, %d, %d), want (255, 136, 0)", r, g, b) }
}

func TestRGBToHex(t *testing.T) {
	got := RGBToHex(255, 136, 0)
	if got != "#ff8800" { t.Errorf("got %q, want %q", got, "#ff8800") }
}

func TestLerpColor(t *testing.T) {
	got := LerpColor("#000000", "#ffffff", 0.5)
	r, g, b, _ := HexToRGB(got)
	if r < 126 || r > 129 || g < 126 || g > 129 || b < 126 || b > 129 {
		t.Errorf("LerpColor black→white 0.5 = %q (%d,%d,%d), want ~#808080", got, r, g, b)
	}
	got0 := LerpColor("#ff0000", "#0000ff", 0.0)
	if got0 != "#ff0000" { t.Errorf("LerpColor at 0.0 = %q, want #ff0000", got0) }
	got1 := LerpColor("#ff0000", "#0000ff", 1.0)
	if got1 != "#0000ff" { t.Errorf("LerpColor at 1.0 = %q, want #0000ff", got1) }
}

func TestLerpFloat(t *testing.T) {
	if got := LerpFloat(0.0, 1.0, 0.5); got != 0.5 { t.Errorf("LerpFloat(0, 1, 0.5) = %f, want 0.5", got) }
	if got := LerpFloat(10.0, 20.0, 0.25); got != 12.5 { t.Errorf("LerpFloat(10, 20, 0.25) = %f, want 12.5", got) }
}
