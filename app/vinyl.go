package app

import (
	"image"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Vinyl mode draws the playing track as a record on a turntable: black
// grooved vinyl with the album art on its centre label, a light reflection
// that stays put while the record turns under it (what makes the spin read
// as a spin), and a tonearm whose needle travels from the edge towards the
// label as the track plays. Pixels are half-block cells, two per character
// row, so they're square.

// vinylRPM is how fast the record turns.
const vinylRPM = 100.0 / 3 // 33⅓

// vinylOmega is vinylRPM in radians per second; vinylFrameEvery paces the
// animation (about 12 frames a second).
const (
	vinylOmega      = vinylRPM * 2 * math.Pi / 60
	vinylFrameEvery = 80 * time.Millisecond
)

// vinylTickMsg turns the record by the time since the last one.
type vinylTickMsg time.Time

func vinylTick() tea.Cmd {
	return tea.Tick(vinylFrameEvery, func(t time.Time) tea.Msg { return vinylTickMsg(t) })
}

// vinylSpinning reports whether the record should be turning: vinyl on
// screen, the track playing, and the terminal focused (each frame is a
// full re-render of the art).
func (m Model) vinylSpinning() bool {
	return m.mode == ModeNowPlaying && m.vinylMode && m.track != nil && m.track.Playing && !m.blurred
}

// startVinylSpin starts the vinyl tick if the record should turn and it
// isn't already.
func (m *Model) startVinylSpin() tea.Cmd {
	if m.vinylTicking || !m.vinylSpinning() {
		return nil
	}
	m.vinylTicking = true
	m.vinylLast = time.Time{}
	return vinylTick()
}

// Record geometry, as fractions of the record's radius.
const (
	vinylLabel   = 0.40 // the label, which carries the album art
	vinylSpindle = 0.035
	vinylLeadIn  = 0.96 // where the needle sets down
	vinylRunOut  = 0.46 // where it ends up, just outside the label
)

// renderVinyl draws the record turned by angle (radians), with the
// tonearm placed for progress (0..1) through the track. bgRows are the
// background colours of the rows the art occupies, so the area around the
// record blends into the gradient behind it; accent tints the reflection.
func renderVinyl(img image.Image, angle, progress float64, w, h int, bgRows []rgb, accent rgb) string {
	if w == 0 || h == 0 || img == nil {
		return ""
	}
	pixels := vinylPixels(img, angle, progress, w, h, bgRows, accent)
	var sb strings.Builder
	sb.Grow(w * h * 30)
	for row := 0; row < h*2; row += 2 {
		for col := range w {
			writeCell(&sb, pixels[row*w+col], pixels[(row+1)*w+col])
		}
		sb.WriteString("\x1b[0m")
		if row+2 < h*2 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// vinylGeometry is where the record and its arm sit in a w×pixH area.
func vinylGeometry(w, pixH int) (cx, cy, radius, size float64) {
	size = math.Min(float64(w), float64(pixH))
	// The record sits left of centre, leaving the top right for the arm.
	return float64(w)/2 - size*0.05, float64(pixH) / 2, size * 0.44, size
}

// vinylPixels computes renderVinyl's pixels, row by row (w × 2h).
func vinylPixels(img image.Image, angle, progress float64, w, h int, bgRows []rgb, accent rgb) []rgb {
	pixH := h * 2
	cx, cy, radius, size := vinylGeometry(w, pixH)
	arm := newTonearm(cx, cy, radius, size, float64(w), progress)

	bounds := img.Bounds()
	srcW, srcH := float64(bounds.Dx()), float64(bounds.Dy())
	srcCX := float64(bounds.Min.X) + srcW/2
	srcCY := float64(bounds.Min.Y) + srcH/2
	labelR := radius * vinylLabel
	// The label shows the middle of the cover, filling the circle.
	artScale := math.Min(srcW, srcH) / (2 * labelR)
	cosA, sinA := math.Cos(angle), math.Sin(angle)

	shine := lerpRGB(rgb{220, 224, 232}, accent, 0.18)
	pixels := make([]rgb, w*pixH)
	for y := range pixH {
		bg := rgb{R: 22, G: 20, B: 24}
		if r := y / 2; r < len(bgRows) {
			bg = bgRows[r]
		}
		for x := range w {
			px, py := float64(x)+0.5, float64(y)+0.5
			dx, dy := px-cx, py-cy
			dist := math.Hypot(dx, dy)

			c := bg
			if cover := radius - dist + 0.5; cover > 0 {
				var disc rgb
				switch {
				case dist < radius*vinylSpindle:
					disc = spindleColor(dx, dy, radius*vinylSpindle)
				case dist < labelR:
					// Inverse-rotate into the cover, so the label turns.
					rx := dx*cosA + dy*sinA
					ry := -dx*sinA + dy*cosA
					disc = sampleBilinear(img, srcCX+rx*artScale, srcCY+ry*artScale)
					if labelR-dist < 0.9 { // a dark ring where label meets vinyl
						disc = scaleRGB(disc, 0.45)
					}
				default:
					disc = grooveColor(dist/radius, math.Atan2(dy, dx), shine)
				}
				c = lerpRGB(bg, disc, math.Min(1, cover))
			}
			c = arm.paint(px, py, c)
			pixels[y*w+x] = c
		}
	}
	return pixels
}

// grooveColor shades the vinyl at r (fraction of the radius) and theta.
// Grooves are concentric, so turning the record doesn't change them; the
// reflection depends only on theta, fixed to the room, not the record.
func grooveColor(r, theta float64, shine rgb) rgb {
	base := 15.0
	// Track gaps: smoother, slightly shinier bands between songs, with soft
	// edges so they don't break into dashes on the pixel grid.
	for _, gap := range [...]float64{0.56, 0.68, 0.80, 0.90} {
		d := (r - gap) / 0.014
		base += 13 * math.Exp(-d*d)
	}
	if r > 0.985 { // the raised rim
		base = 46
	}
	// Two opposite wedges of reflected light, from a light at the top left.
	s := math.Pow(math.Abs(math.Cos(theta+math.Pi/4)), 14)
	s *= 0.55 + 0.45*r // stronger towards the edge, where the grooves are wider
	g := rgb{R: uint8(base), G: uint8(base), B: uint8(base + 2)}
	return lerpRGB(g, shine, s*0.24)
}

// spindleColor is the metal spindle, lit from the top left.
func spindleColor(dx, dy, r float64) rgb {
	l := 0.5 - 0.5*(dx+dy)/(2*r) // brighter towards the top left
	v := uint8(120 + 110*math.Max(0, math.Min(1, l)))
	return rgb{R: v, G: v, B: v + 4}
}

// tonearm is the arm drawn over the record: a pivot outside the record's
// top-right edge and a straight arm to a needle that moves inwards with the
// track's progress.
type tonearm struct {
	pivotX, pivotY float64
	tipX, tipY     float64
	width          float64
}

func newTonearm(cx, cy, radius, size, w, progress float64) tonearm {
	progress = math.Max(0, math.Min(1, progress))
	pivotX := math.Min(w-size*0.06, cx+radius*1.12)
	pivotY := cy - radius*0.92
	// The arm has a fixed length: it reaches the lead-in groove a little
	// below the record's right-hand side, and swings in an arc from there.
	leadIn := radius * vinylLeadIn
	start := 0.18 // radians below horizontal
	length := math.Hypot(cx+leadIn*math.Cos(start)-pivotX, cy+leadIn*math.Sin(start)-pivotY)
	// The needle is where the arm's circle crosses the groove at needleR.
	needleR := radius * (vinylLeadIn + (vinylRunOut-vinylLeadIn)*progress)
	tipX, tipY := circleCross(cx, cy, needleR, pivotX, pivotY, length)
	return tonearm{
		pivotX: pivotX, pivotY: pivotY,
		tipX: tipX, tipY: tipY,
		width: math.Max(1.2, size*0.018),
	}
}

// circleCross is where the circle of radius r1 around (x1, y1) meets the
// one of radius r2 around (x2, y2): the crossing on the right of the line
// between the centres (the record's side under the arm), or the nearest
// point of the first circle when they don't meet.
func circleCross(x1, y1, r1, x2, y2, r2 float64) (float64, float64) {
	dx, dy := x2-x1, y2-y1
	d := math.Hypot(dx, dy)
	if d == 0 {
		return x1 + r1, y1
	}
	a := (r1*r1 - r2*r2 + d*d) / (2 * d)
	hh := r1*r1 - a*a
	if hh < 0 {
		return x1 + dx/d*r1, y1 + dy/d*r1
	}
	h := math.Sqrt(hh)
	mx, my := x1+a*dx/d, y1+a*dy/d
	// Of the two crossings, the one further clockwise from the pivot
	// (lower right on screen).
	px, py := mx-h*dy/d, my+h*dx/d
	qx, qy := mx+h*dy/d, my-h*dx/d
	if py > qy {
		return px, py
	}
	return qx, qy
}

// paint draws the arm (and its pivot) over c at pixel (x, y).
func (a tonearm) paint(x, y float64, c rgb) rgb {
	metal := rgb{R: 190, G: 190, B: 196}
	if d := math.Hypot(x-a.pivotX, y-a.pivotY); d < a.width*1.7 {
		return lerpRGB(c, rgb{R: 165, G: 165, B: 172}, math.Min(1, a.width*1.7-d+0.5))
	}
	// Distance from the segment pivot → tip.
	vx, vy := a.tipX-a.pivotX, a.tipY-a.pivotY
	t := ((x-a.pivotX)*vx + (y-a.pivotY)*vy) / (vx*vx + vy*vy)
	if t < 0 || t > 1 {
		return c
	}
	d := math.Hypot(x-(a.pivotX+t*vx), y-(a.pivotY+t*vy))
	head := t > 0.9 // the headshell is a little wider
	half := a.width / 2
	if head {
		half *= 1.9
	}
	if cover := half - d + 0.5; cover > 0 {
		return lerpRGB(c, metal, math.Min(1, cover))
	}
	return c
}

func sampleBilinear(img image.Image, fx, fy float64) rgb {
	b := img.Bounds()
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := fx-float64(x0), fy-float64(y0)
	at := func(x, y int) (uint32, uint32, uint32) {
		r, g, bl, _ := img.At(clampInt(x, b.Min.X, b.Max.X-1), clampInt(y, b.Min.Y, b.Max.Y-1)).RGBA()
		return r, g, bl
	}
	r00, g00, b00 := at(x0, y0)
	r10, g10, b10 := at(x0+1, y0)
	r01, g01, b01 := at(x0, y0+1)
	r11, g11, b11 := at(x0+1, y0+1)
	return rgb{
		R: blerp(r00, r10, r01, r11, tx, ty),
		G: blerp(g00, g10, g01, g11, tx, ty),
		B: blerp(b00, b10, b01, b11, tx, ty),
	}
}

func lerpRGB(a, b rgb, t float64) rgb {
	t = math.Max(0, math.Min(1, t))
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return rgb{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B)}
}

func scaleRGB(c rgb, f float64) rgb {
	return rgb{R: uint8(float64(c.R) * f), G: uint8(float64(c.G) * f), B: uint8(float64(c.B) * f)}
}
