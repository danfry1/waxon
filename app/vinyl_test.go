package app

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// quadrants is a cover with four differently coloured corners, so turning
// it changes what the label shows.
func quadrants() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	cols := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}}
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, cols[(y/32)*2+x/32])
		}
	}
	return img
}

func TestVinylSize(t *testing.T) {
	out := renderVinyl(quadrants(), 0, 0.3, 40, 20, nil, rgb{200, 50, 50})
	if lines := strings.Split(out, "\n"); len(lines) != 20 {
		t.Errorf("%d rows, want 20", len(lines))
	}
}

// The label turns with the record; the grooves and the reflection on them
// don't, which is what makes the spin read as a spin.
func TestVinylLabelTurnsGroovesDont(t *testing.T) {
	const w, h = 60, 30
	a := vinylPixels(quadrants(), 0, 0, w, h, nil, rgb{})
	b := vinylPixels(quadrants(), 1.1, 0, w, h, nil, rgb{})
	cx, cy, radius, _ := vinylGeometry(w, h*2)
	labelDiffs, grooveDiffs := 0, 0
	for y := range h * 2 {
		for x := range w {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) / radius
			if a[y*w+x] == b[y*w+x] {
				continue
			}
			switch {
			case d < vinylLabel*0.9 && d > vinylSpindle*2:
				labelDiffs++
			case d > vinylLabel*1.1 && d < 0.95:
				grooveDiffs++
			}
		}
	}
	if labelDiffs == 0 {
		t.Error("turning the record didn't change the label")
	}
	if grooveDiffs != 0 {
		t.Errorf("turning the record changed %d groove pixels; grooves and the reflection should stay put", grooveDiffs)
	}
}

// The needle sets down at the lead-in and moves steadily in to the run-out,
// staying on the record.
func TestTonearmFollowsProgress(t *testing.T) {
	cx, cy, radius, size := vinylGeometry(60, 60)
	prev := math.Inf(1)
	for _, p := range []float64{0, 0.25, 0.5, 0.75, 1} {
		a := newTonearm(cx, cy, radius, size, 60, p)
		r := math.Hypot(a.tipX-cx, a.tipY-cy) / radius
		want := vinylLeadIn + (vinylRunOut-vinylLeadIn)*p
		if math.Abs(r-want) > 0.02 {
			t.Errorf("progress %.2f: needle at %.3f of the radius, want %.3f", p, r, want)
		}
		if r >= prev {
			t.Errorf("progress %.2f: needle didn't move inwards (%.3f after %.3f)", p, r, prev)
		}
		prev = r
	}
}

func TestVinylSpinsAt33AndStopsWhenPaused(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.track = &source.Track{ID: "t", Playing: true}
	m.mode, m.vinylMode = ModeNowPlaying, true
	if m.startVinylSpin() == nil {
		t.Fatal("vinyl on screen and playing should start the spin")
	}
	start := time.Now()
	var cmd tea.Cmd
	for i := 0; i <= 12; i++ { // a second at the real frame rate (12 × 83ms ≈ 1s)
		var res tea.Model
		res, cmd = m.Update(vinylTickMsg(start.Add(time.Duration(i) * time.Second / 12)))
		m = res.(Model)
	}
	if want := vinylOmega; math.Abs(m.vinylAngle-want) > 1e-6 {
		t.Errorf("after 1s the record turned %.3f rad, want %.3f (33⅓ rpm)", m.vinylAngle, want)
	}
	if cmd == nil {
		t.Error("the spin should keep ticking while playing")
	}
	m.track.Playing = false
	res, cmd := m.Update(vinylTickMsg(start.Add(2 * time.Second)))
	if m = res.(Model); cmd != nil || m.vinylTicking {
		t.Error("a paused record should stop ticking")
	}
}
