package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"sync"
	"time"
)

// Album art via the kitty graphics protocol's Unicode placeholders (kitty,
// Ghostty). The image is sent to the terminal once; the view then contains
// ordinary text cells (U+10EEEE plus diacritics, coloured with the image ID)
// that the terminal replaces with the picture. Because the placeholders are
// text, Bubbletea's line renderer handles them like any other output.
//
// Images have to be written to the terminal outside the view, between
// frames. TTYOutput is the Program's output: it serialises the renderer's
// frame writes with ours so an image never lands inside a frame.

// TTYOutput wraps the terminal so graphics can be interleaved with frames.
type TTYOutput struct {
	f  *os.File
	mu sync.Mutex
}

// NewTTYOutput wraps f (normally os.Stdout) for use with tea.WithOutput.
func NewTTYOutput(f *os.File) *TTYOutput { return &TTYOutput{f: f} }

func (t *TTYOutput) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Write(p)
}

func (t *TTYOutput) Read(p []byte) (int, error) { return t.f.Read(p) }
func (t *TTYOutput) Close() error               { return t.f.Close() }
func (t *TTYOutput) Fd() uintptr                { return t.f.Fd() }

var graphicsOut *TTYOutput

// SetGraphicsOutput enables kitty art: images are written to out, which must
// also be the Program's output. Until it's set, ArtKitty falls back to
// half-blocks.
func SetGraphicsOutput(out *TTYOutput) { graphicsOut = out }

// KittyTerminal reports whether the terminal is known to support kitty
// Unicode placeholders. Inside tmux the variables leak through from the
// outer terminal, but the images wouldn't (they need passthrough), so tmux
// counts as unsupported, as do zellij and screen.
func KittyTerminal() bool {
	if os.Getenv("TMUX") != "" || os.Getenv("ZELLIJ") != "" || os.Getenv("STY") != "" {
		return false
	}
	if t := os.Getenv("TERM"); strings.HasPrefix(t, "screen") || strings.HasPrefix(t, "tmux") {
		return false
	}
	return os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" ||
		strings.EqualFold(os.Getenv("TERM_PROGRAM"), "ghostty")
}

// kittyMaxPx caps the edge of the image sent to the terminal. The terminal
// scales it to the placement, so this only bounds the transfer size.
const kittyMaxPx = 480

// kittyImageBase offsets waxon's image IDs so they're unlikely to collide
// with other programs' in the same terminal. IDs must fit in 24 bits to be
// expressed as a true-colour foreground.
const kittyImageBase = 0x5A0000

var kitty struct {
	mu      sync.Mutex
	next    uint32
	slots   map[string]kittySlot // slot ("status", "np") → what it shows
	pngs    map[image.Image][]byte
	pending []kittyDeletion // replaced images, deleted once off screen
}

// kittySlot is the image an art area currently shows.
type kittySlot struct {
	id          uint32
	img         image.Image
	cols, rows  int
	placeholder string
}

type kittyDeletion struct {
	id uint32
	at time.Time
}

// kittyDeleteDelay is how long a replaced image is kept: deleting it at once
// would blank its cells until the next frame replaces them.
const kittyDeleteDelay = 300 * time.Millisecond

// kittyArt sends img to the terminal for a cols×rows area and returns the
// placeholder text to draw there. slot names the place the art occupies;
// the image previously shown in that slot is deleted.
func kittyArt(slot string, img image.Image, cols, rows int) string {
	if graphicsOut == nil || img == nil || cols <= 0 || rows <= 0 || rows > len(kittyRowDiacritics) {
		return ""
	}
	kitty.mu.Lock()
	defer kitty.mu.Unlock()
	if kitty.slots == nil {
		kitty.slots = map[string]kittySlot{}
		kitty.pngs = map[image.Image][]byte{}
	}
	old, hadOld := kitty.slots[slot]
	if hadOld && old.img == img && old.cols == cols && old.rows == rows {
		return old.placeholder // already on the terminal
	}
	data, ok := kitty.pngs[img]
	if !ok {
		var err error
		if data, err = kittyPNG(img); err != nil {
			return ""
		}
		if len(kitty.pngs) >= 4 { // the status and NP art, plus the next track's
			clear(kitty.pngs)
		}
		kitty.pngs[img] = data
	}
	kitty.next = (kitty.next + 1) % 0xFFFF
	id := kittyImageBase + kitty.next
	ph := kittyPlaceholders(id, cols, rows)
	kitty.slots[slot] = kittySlot{id: id, img: img, cols: cols, rows: rows, placeholder: ph}
	if hadOld {
		kitty.pending = append(kitty.pending, kittyDeletion{id: old.id, at: time.Now()})
	}
	var seq strings.Builder
	writeKittyTransmit(&seq, id, cols, rows, data)
	_, _ = graphicsOut.Write([]byte(seq.String()))
	return ph
}

// kittyRelease frees the image shown in slot (its art was cleared).
func kittyRelease(slot string) {
	kitty.mu.Lock()
	defer kitty.mu.Unlock()
	if s, ok := kitty.slots[slot]; ok {
		delete(kitty.slots, slot)
		kitty.pending = append(kitty.pending, kittyDeletion{id: s.id, at: time.Now()})
	}
}

// kittyReap deletes replaced images once the frame that stopped showing them
// has had time to reach the terminal. Called from Update.
func kittyReap(now time.Time) {
	kitty.mu.Lock()
	defer kitty.mu.Unlock()
	if graphicsOut == nil || len(kitty.pending) == 0 {
		return
	}
	var seq strings.Builder
	keep := kitty.pending[:0]
	for _, d := range kitty.pending {
		if now.Sub(d.at) < kittyDeleteDelay {
			keep = append(keep, d)
			continue
		}
		fmt.Fprintf(&seq, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", d.id)
	}
	kitty.pending = keep
	if seq.Len() > 0 {
		_, _ = graphicsOut.Write([]byte(seq.String()))
	}
}

// kittyClear deletes every image waxon has placed. It runs as waxon quits,
// while still on the alternate screen where the images live.
func kittyClear() {
	kitty.mu.Lock()
	defer kitty.mu.Unlock()
	if graphicsOut == nil {
		return
	}
	var seq strings.Builder
	for slot, s := range kitty.slots {
		fmt.Fprintf(&seq, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", s.id)
		delete(kitty.slots, slot)
	}
	for _, d := range kitty.pending {
		fmt.Fprintf(&seq, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", d.id)
	}
	kitty.pending = nil
	_, _ = graphicsOut.Write([]byte(seq.String()))
}

func kittyPNG(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if w, h := b.Dx(), b.Dy(); w > kittyMaxPx || h > kittyMaxPx {
		scale := float64(kittyMaxPx) / float64(max(w, h))
		dw, dh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
		px := scaleBilinear(img, dw, dh)
		dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
		for y := range dh {
			for x := range dw {
				c := px[y][x]
				dst.SetRGBA(x, y, color.RGBA{c.R, c.G, c.B, 0xFF})
			}
		}
		img = dst
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeKittyTransmit sends PNG data and creates a virtual placement (U=1)
// of cols×rows cells in one go, in base64 chunks of at most 4096 bytes.
// q=2 suppresses the terminal's replies, which would otherwise arrive as
// input.
func writeKittyTransmit(sb *strings.Builder, id uint32, cols, rows int, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	const chunk = 4096
	for i := 0; i < len(enc); i += chunk {
		end := min(i+chunk, len(enc))
		more := 1
		if end == len(enc) {
			more = 0
		}
		if i == 0 {
			fmt.Fprintf(sb, "\x1b_Ga=T,U=1,f=100,t=d,i=%d,c=%d,r=%d,q=2,m=%d;", id, cols, rows, more)
		} else {
			fmt.Fprintf(sb, "\x1b_Gm=%d;", more)
		}
		sb.WriteString(enc[i:end])
		sb.WriteString("\x1b\\")
	}
}

// kittyPlaceholders draws the cells for image id: the first cell of each
// row carries row and column diacritics; the rest inherit them from the
// cell to their left (column + 1), which keeps each row short.
func kittyPlaceholders(id uint32, cols, rows int) string {
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (id>>16)&0xFF, (id>>8)&0xFF, id&0xFF)
	var sb strings.Builder
	for r := range rows {
		if r > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(fg)
		sb.WriteRune(kittyPlaceholder)
		sb.WriteRune(kittyRowDiacritics[r])
		sb.WriteRune(kittyRowDiacritics[0]) // column 0
		for range cols - 1 {
			sb.WriteRune(kittyPlaceholder)
		}
		sb.WriteString("\x1b[39m")
	}
	return sb.String()
}

const kittyPlaceholder = '\U0010EEEE'

// kittyRowDiacritics are the combining marks that number placeholder rows
// and columns (0, 1, 2, …), from kitty's rowcolumn-diacritics.txt.
var kittyRowDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A,
	0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365,
	0x0366, 0x0367, 0x0368, 0x0369, 0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F,
	0x0483, 0x0484, 0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1, 0x05A8, 0x05A9,
	0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611, 0x0612, 0x0613, 0x0614, 0x0615,
	0x0616, 0x0617, 0x0657, 0x0658, 0x0659, 0x065A, 0x065B, 0x065D, 0x065E, 0x06D6,
	0x06D7, 0x06D8, 0x06D9, 0x06DA, 0x06DB, 0x06DC, 0x06DF, 0x06E0, 0x06E1, 0x06E2,
	0x06E4, 0x06E7, 0x06E8, 0x06EB, 0x06EC, 0x0730, 0x0732, 0x0733, 0x0735, 0x0736,
	0x073A, 0x073D, 0x073F, 0x0740, 0x0741, 0x0743, 0x0745, 0x0747, 0x0749, 0x074A,
}
