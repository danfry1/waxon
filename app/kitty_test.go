package app

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// withKittyOutput routes kitty writes to a temp file for the test and returns
// a function that reads back everything written so far.
func withKittyOutput(t *testing.T) func() string {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "tty"))
	if err != nil {
		t.Fatal(err)
	}
	prevOut, prevMode := graphicsOut, artMode
	SetGraphicsOutput(NewTTYOutput(f))
	SetArtMode(ArtKitty)
	kitty.slots = nil
	t.Cleanup(func() {
		graphicsOut, artMode = prevOut, prevMode
		kitty.slots = nil
		f.Close()
	})
	return func() string {
		b, _ := os.ReadFile(f.Name())
		return string(b)
	}
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 99, 255})
		}
	}
	return img
}

func TestKittyArtPlaceholdersAndTransmit(t *testing.T) {
	written := withKittyOutput(t)
	out := renderArt("status", testImage(900, 900), 6, 3)

	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d placeholder rows, want 3", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 6 {
			t.Errorf("row %d is %d cells wide, want 6", i, w)
		}
		if !strings.HasPrefix(ansi.Strip(l), string(kittyPlaceholder)+string(kittyRowDiacritics[i])+string(kittyRowDiacritics[0])) {
			t.Errorf("row %d should start with its row and column diacritics", i)
		}
	}

	seq := written()
	head := regexp.MustCompile(`\x1b_Ga=T,U=1,f=100,t=d,i=(\d+),c=6,r=3,q=2,m=[01];`).FindStringSubmatch(seq)
	if head == nil {
		t.Fatalf("no transmit+placement command in %q…", seq[:min(80, len(seq))])
	}
	// Every chunk is a complete APC: ESC _ G … ESC \, payload ≤ 4096.
	for _, c := range regexp.MustCompile(`\x1b_G[^;]*;([^\x1b]*)\x1b\\`).FindAllStringSubmatch(seq, -1) {
		if len(c[1]) > 4096 {
			t.Errorf("chunk of %d bytes exceeds 4096", len(c[1]))
		}
	}
	if !strings.HasSuffix(seq, "\x1b\\") || strings.Count(seq, "m=0;") != 1 {
		t.Error("transmission should end with exactly one final (m=0) chunk")
	}
}

func TestKittyArtReplacesSlotImage(t *testing.T) {
	written := withKittyOutput(t)
	renderArt("status", testImage(10, 10), 4, 2)
	first := kitty.slots["status"].id
	renderArt("status", testImage(10, 10), 4, 2)
	if kitty.slots["status"].id == first {
		t.Fatal("a new image should get a new ID")
	}
	del := "\x1b_Ga=d,d=I,i=" + itoa(first) + ",q=2\x1b\\"
	if strings.Contains(written(), del) {
		t.Error("the replaced image must stay until the next frame has drawn over it")
	}
	kittyReap(time.Now())
	if strings.Contains(written(), del) {
		t.Error("too soon to delete")
	}
	kittyReap(time.Now().Add(time.Second))
	if !strings.Contains(written(), del) {
		t.Error("the replaced image should be deleted once off screen")
	}
	renderArt("np", testImage(10, 10), 4, 2)
	if len(kitty.slots) != 2 {
		t.Errorf("slots = %v, want status and np", kitty.slots)
	}
}

func TestKittyArtUnchangedIsNotResent(t *testing.T) {
	written := withKittyOutput(t)
	img := testImage(10, 10)
	a := renderArt("np", img, 4, 2)
	n := len(written())
	if b := renderArt("np", img, 4, 2); b != a || len(written()) != n {
		t.Error("the same image at the same size should reuse its placement")
	}
	renderArt("np", img, 6, 3)
	if strings.Count(written(), "a=T,") != 2 {
		t.Error("a new size needs a new placement")
	}
}

func TestKittyNPArtOnlyWhileVisible(t *testing.T) {
	written := withKittyOutput(t)
	m := newTestModel(&StubSource{})
	m.npSourceImg = testImage(10, 10)
	m.renderNPArt()
	if strings.Contains(written(), "a=T,") {
		t.Error("Now Playing art shouldn't be sent while the view is closed")
	}
	m.mode = ModeNowPlaying
	m.renderNPArt()
	if !strings.Contains(written(), "a=T,") {
		t.Error("Now Playing art should be sent once the view is open")
	}
}

func TestKittyTerminalDetection(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		{map[string]string{"KITTY_WINDOW_ID": "1"}, true},
		{map[string]string{"KITTY_WINDOW_ID": "1", "TMUX": "/tmp/x"}, false},
		{map[string]string{"TERM_PROGRAM": "ghostty", "ZELLIJ": "0"}, false},
		{map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "screen-256color"}, false},
		{map[string]string{"TERM": "xterm-256color"}, false},
	} {
		for _, k := range []string{"TERM_PROGRAM", "KITTY_WINDOW_ID", "TMUX", "ZELLIJ", "STY", "TERM"} {
			t.Setenv(k, tc.env[k])
		}
		if got := KittyTerminal(); got != tc.want {
			t.Errorf("%v: KittyTerminal() = %v", tc.env, got)
		}
	}
}

func TestKittyFallsBackWithoutOutput(t *testing.T) {
	prevOut, prevMode := graphicsOut, artMode
	defer func() { graphicsOut, artMode = prevOut, prevMode }()
	graphicsOut = nil
	SetArtMode(ArtKitty)
	if out := renderArt("status", testImage(10, 10), 4, 2); !strings.Contains(out, "▀") {
		t.Error("without a graphics output, kitty mode should draw half-blocks")
	}
}

func itoa(n uint32) string { return strconv.FormatUint(uint64(n), 10) }
