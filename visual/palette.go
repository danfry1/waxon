package visual

import (
	"fmt"
	"math"
	"strings"
)

func HexToRGB(hex string) (uint8, uint8, uint8, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %q", hex)
	}
	var r, g, b uint8
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b, err
}

func RGBToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func LerpColor(from, to string, t float64) string {
	t = math.Max(0, math.Min(1, t))
	r1, g1, b1, err1 := HexToRGB(from)
	r2, g2, b2, err2 := HexToRGB(to)
	if err1 != nil || err2 != nil {
		return from
	}
	r := uint8(math.Round(float64(r1) + t*(float64(r2)-float64(r1))))
	g := uint8(math.Round(float64(g1) + t*(float64(g2)-float64(g1))))
	b := uint8(math.Round(float64(b1) + t*(float64(b2)-float64(b1))))
	return RGBToHex(r, g, b)
}

func LerpFloat(from, to, t float64) float64 {
	return from + t*(to-from)
}
