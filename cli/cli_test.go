package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danfry1/waxon/source"
)

// stub is a partial RichSource: methods not overridden panic via the nil
// embedded interface, which is exactly what we want — a test that hits an
// unexpected endpoint fails loudly.
type stub struct {
	source.RichSource
	state   *source.PlaybackState
	stateFn func() (*source.PlaybackState, error)
	devices []source.Device
	saved   map[string]bool
	results *source.SearchResults
	calls   []string
	noDev   bool // playback commands fail with ErrNoActiveDevice until transfer
}

func (s *stub) rec(f string, a ...any) {
	parts := make([]string, 0, len(a)+1)
	parts = append(parts, f)
	for _, v := range a {
		parts = append(parts, fmt.Sprint(v))
	}
	s.calls = append(s.calls, strings.TrimSpace(strings.Join(parts, " ")))
}

func (s *stub) dev() error {
	if s.noDev {
		return source.ErrNoActiveDevice
	}
	return nil
}

func (s *stub) CurrentPlayback(context.Context) (*source.PlaybackState, error) {
	if s.stateFn != nil {
		return s.stateFn()
	}
	return s.state, nil
}
func (s *stub) Play(context.Context) error     { s.rec("play"); return s.dev() }
func (s *stub) Pause(context.Context) error    { s.rec("pause"); return s.dev() }
func (s *stub) Next(context.Context) error     { s.rec("next"); return s.dev() }
func (s *stub) Previous(context.Context) error { s.rec("prev"); return s.dev() }
func (s *stub) Seek(_ context.Context, p time.Duration) error {
	s.rec("seek", p.Seconds())
	return s.dev()
}
func (s *stub) SetVolume(_ context.Context, v int) error   { s.rec("vol", v); return s.dev() }
func (s *stub) SetShuffle(_ context.Context, b bool) error { s.rec("shuffle", b); return s.dev() }
func (s *stub) SetRepeat(_ context.Context, m source.RepeatMode) error {
	s.rec("repeat", m)
	return s.dev()
}
func (s *stub) Devices(context.Context) ([]source.Device, error) { return s.devices, nil }
func (s *stub) TransferPlayback(_ context.Context, id string) error {
	s.rec("transfer", id)
	s.noDev = false
	return nil
}

func (s *stub) PlayTrack(_ context.Context, c, t string) error {
	s.rec("playtrack", c, t)
	return s.dev()
}

func (s *stub) PlayTrackDirect(_ context.Context, t string) error {
	s.rec("playdirect", t)
	return s.dev()
}
func (s *stub) AddToQueue(_ context.Context, id string) error { s.rec("queue", id); return s.dev() }
func (s *stub) Search(_ context.Context, q string) (*source.SearchResults, error) {
	s.rec("search", q)
	return s.results, nil
}
func (s *stub) IsTrackSaved(_ context.Context, id string) (bool, error) { return s.saved[id], nil }
func (s *stub) SaveTrack(_ context.Context, id string) error {
	s.rec("save", id)
	s.saved[id] = true
	return nil
}

func (s *stub) RemoveTrack(_ context.Context, id string) error {
	s.rec("remove", id)
	delete(s.saved, id)
	return nil
}

func playing() *source.PlaybackState {
	return &source.PlaybackState{
		Track: &source.Track{
			ID: "t1", URI: "spotify:track:t1", Name: "Song", Artist: "Band", Album: "LP",
			Position: 90 * time.Second, Duration: 4 * time.Minute, Playing: true, DeviceName: "Laptop",
		},
		Volume: 55, ShuffleOn: true, RepeatMode: source.RepeatContext,
	}
}

func run(t *testing.T, s *stub, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	r := &Runner{Src: s, Out: &o, Err: &e, Sleep: func(time.Duration) {}}
	code = r.Run(context.Background(), args)
	return code, o.String(), e.String()
}

func TestStatusDefaultFormat(t *testing.T) {
	code, out, _ := run(t, &stub{state: playing()}, "status")
	if code != ExitOK || out != "▶ Song — Band\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestStatusCustomFormat(t *testing.T) {
	_, out, _ := run(t, &stub{state: playing()}, "status", "--format",
		"{state}|{position}/{duration}|{progress}|{volume}|{shuffle}|{repeat}|{device}|{album}|{uri}")
	want := "playing|1:30/4:00|37|55|on|context|Laptop|LP|spotify:track:t1\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestStatusPausedIcon(t *testing.T) {
	st := playing()
	st.Track.Playing = false
	_, out, _ := run(t, &stub{state: st}, "status")
	if !strings.HasPrefix(out, "⏸ ") {
		t.Fatalf("got %q", out)
	}
}

func TestStatusNothingPlayingPrintsNothing(t *testing.T) {
	code, out, _ := run(t, &stub{}, "status")
	if code != ExitOK || out != "" {
		t.Fatalf("code=%d out=%q; idle status should be blank for status bars", code, out)
	}
	_, out, _ = run(t, &stub{}, "status", "--json")
	var js map[string]any
	if err := json.Unmarshal([]byte(out), &js); err != nil || js["playing"] != false {
		t.Fatalf("idle --json should still be valid JSON with playing=false: %q", out)
	}
}

func TestStatusJSON(t *testing.T) {
	s := &stub{state: playing(), saved: map[string]bool{"t1": true}}
	_, out, _ := run(t, s, "status", "--json", "--liked")
	var js statusJSON
	if err := json.Unmarshal([]byte(out), &js); err != nil {
		t.Fatal(err)
	}
	if !js.Playing || js.Title != "Song" || js.PositionMS != 90000 || js.DurationMS != 240000 ||
		js.Progress != 37 || js.Volume != 55 || !js.Shuffle || js.Repeat != "context" || js.Liked == nil || !*js.Liked {
		t.Fatalf("unexpected JSON: %+v", js)
	}
}

func TestStatusWaybar(t *testing.T) {
	st := playing()
	st.Track.Name = `Say "Hi"` // quotes must be JSON-escaped, not break the object
	_, out, _ := run(t, &stub{state: st}, "status", "--waybar")
	var wb waybarJSON
	if err := json.Unmarshal([]byte(out), &wb); err != nil {
		t.Fatalf("invalid waybar JSON %q: %v", out, err)
	}
	if wb.Text != `Say "Hi" — Band` || wb.Alt != "playing" || wb.Class != "playing" || wb.Percentage != 37 ||
		!strings.Contains(wb.Tooltip, "1:30 / 4:00") {
		t.Errorf("unexpected waybar payload: %+v", wb)
	}
	// Custom --format is honoured for the text field.
	_, out, _ = run(t, &stub{state: st}, "status", "--waybar", "--format", "{artist}")
	_ = json.Unmarshal([]byte(out), &wb)
	if wb.Text != "Band" {
		t.Errorf("text = %q, want custom format", wb.Text)
	}
	// Idle still emits a valid object so waybar can hide/style it.
	_, out, _ = run(t, &stub{}, "status", "--waybar")
	if err := json.Unmarshal([]byte(out), &wb); err != nil || wb.Class != "idle" || wb.Text != "" {
		t.Errorf("idle waybar payload %q (%v)", out, err)
	}
}

func TestStatusLikedPlaceholderRequiresFlag(t *testing.T) {
	s := &stub{state: playing(), saved: map[string]bool{"t1": true}}
	_, out, _ := run(t, s, "status", "--format", "[{liked}]")
	if out != "[]\n" {
		t.Fatalf("without --liked the placeholder should be empty, got %q", out)
	}
	_, out, _ = run(t, s, "status", "--liked", "--format", "[{liked}]")
	if out != "[♥]\n" {
		t.Fatalf("got %q", out)
	}
}

func TestSimpleControls(t *testing.T) {
	for _, c := range []struct{ cmd, want string }{
		{"play", "play"}, {"pause", "pause"}, {"next", "next"}, {"prev", "prev"},
	} {
		s := &stub{state: playing()}
		if code, _, e := run(t, s, c.cmd); code != ExitOK {
			t.Fatalf("%s: code=%d err=%q", c.cmd, code, e)
		}
		if len(s.calls) != 1 || s.calls[0] != c.want {
			t.Errorf("%s: calls=%v", c.cmd, s.calls)
		}
	}
}

func TestToggle(t *testing.T) {
	s := &stub{state: playing()}
	run(t, s, "toggle")
	if s.calls[len(s.calls)-1] != "pause" {
		t.Errorf("playing → toggle should pause, calls=%v", s.calls)
	}
	st := playing()
	st.Track.Playing = false
	s = &stub{state: st}
	run(t, s, "toggle")
	if s.calls[len(s.calls)-1] != "play" {
		t.Errorf("paused → toggle should play, calls=%v", s.calls)
	}
	s = &stub{}
	run(t, s, "toggle")
	if s.calls[len(s.calls)-1] != "play" {
		t.Errorf("idle → toggle should play, calls=%v", s.calls)
	}
}

func TestSeek(t *testing.T) {
	cases := []struct{ arg, want string }{
		{"30", "seek 30"},
		{"1:30", "seek 90"},
		{"1:02:03", "seek 3723"},
		{"+10", "seek 100"},
		{"-10", "seek 80"},
		{"-500", "seek 0"},
		{"+1000", "seek 240"},
	}
	for _, c := range cases {
		s := &stub{state: playing()}
		if code, _, e := run(t, s, "seek", c.arg); code != ExitOK {
			t.Fatalf("seek %s: code=%d err=%q", c.arg, code, e)
		}
		if got := s.calls[len(s.calls)-1]; got != c.want {
			t.Errorf("seek %s: got %q want %q", c.arg, got, c.want)
		}
	}
	if code, _, _ := run(t, &stub{state: playing()}, "seek", "abc"); code != ExitUsage {
		t.Error("bad seek arg should be a usage error")
	}
	if code, _, _ := run(t, &stub{}, "seek", "+10"); code != ExitError {
		t.Error("relative seek with nothing playing should fail")
	}
}

func TestVol(t *testing.T) {
	cases := []struct{ arg, want, out string }{
		{"30", "vol 30", "volume 30%\n"},
		{"+10", "vol 65", "volume 65%\n"},
		{"-60", "vol 0", "volume 0%\n"},
		{"+90", "vol 100", "volume 100%\n"},
		{"150", "vol 100", "volume 100%\n"},
	}
	for _, c := range cases {
		s := &stub{state: playing()}
		_, out, _ := run(t, s, "vol", c.arg)
		if got := s.calls[len(s.calls)-1]; got != c.want || out != c.out {
			t.Errorf("vol %s: call=%q out=%q", c.arg, got, out)
		}
	}
	if code, _, _ := run(t, &stub{}, "vol", "loud"); code != ExitUsage {
		t.Error("bad vol arg should be a usage error")
	}
}

func TestShuffleRepeat(t *testing.T) {
	s := &stub{state: playing()} // shuffle currently on
	run(t, s, "shuffle")
	if s.calls[len(s.calls)-1] != "shuffle false" {
		t.Errorf("toggle should turn shuffle off, calls=%v", s.calls)
	}
	run(t, s, "shuffle", "on")
	if s.calls[len(s.calls)-1] != "shuffle true" {
		t.Errorf("calls=%v", s.calls)
	}
	run(t, s, "repeat", "one")
	if s.calls[len(s.calls)-1] != "repeat track" {
		t.Errorf("calls=%v", s.calls)
	}
	if code, _, _ := run(t, s, "repeat", "sometimes"); code != ExitUsage {
		t.Error("bad repeat mode should be a usage error")
	}
}

func TestLike(t *testing.T) {
	s := &stub{state: playing(), saved: map[string]bool{}}
	_, out, _ := run(t, s, "like")
	if !s.saved["t1"] || !strings.Contains(out, "liked") {
		t.Fatalf("like should save: saved=%v out=%q", s.saved, out)
	}
	_, out, _ = run(t, s, "like")
	if s.saved["t1"] || !strings.Contains(out, "unliked") {
		t.Fatalf("second like should toggle off: out=%q", out)
	}
	_, out, _ = run(t, s, "like", "off")
	if !strings.Contains(out, "already unliked") {
		t.Fatalf("explicit no-op should say so: %q", out)
	}
	if code, _, e := run(t, &stub{saved: map[string]bool{}}, "like"); code != ExitError || !strings.Contains(e, "nothing is playing") {
		t.Errorf("like with nothing playing: code=%d err=%q", code, e)
	}
}

func TestPlayAndQueueBySearch(t *testing.T) {
	res := &source.SearchResults{Tracks: []source.Track{
		{ID: "x1", URI: "spotify:track:x1", Name: "Hit", Artist: "Star", AlbumID: "al1"},
	}}
	s := &stub{state: playing(), results: res}
	_, out, _ := run(t, s, "play", "hit", "song")
	if s.calls[0] != "search hit song" || s.calls[1] != "playtrack spotify:album:al1 spotify:track:x1" {
		t.Errorf("play by search: calls=%v", s.calls)
	}
	if !strings.Contains(out, "playing Hit — Star") {
		t.Errorf("out=%q", out)
	}
	s = &stub{state: playing(), results: res}
	run(t, s, "queue", "hit")
	if s.calls[len(s.calls)-1] != "queue x1" {
		t.Errorf("queue by search: calls=%v", s.calls)
	}
	s = &stub{state: playing()}
	run(t, s, "queue", "spotify:track:abc")
	if s.calls[0] != "queue abc" {
		t.Errorf("queue by uri: calls=%v", s.calls)
	}
	s = &stub{state: playing()}
	run(t, s, "play", "spotify:album:zzz")
	if s.calls[0] != "playtrack spotify:album:zzz" {
		t.Errorf("play by context uri: calls=%v", s.calls)
	}
	s = &stub{results: &source.SearchResults{}}
	if code, _, e := run(t, s, "play", "nothing"); code != ExitError || !strings.Contains(e, "no tracks found") {
		t.Errorf("empty search: code=%d err=%q", code, e)
	}
}

func TestSearchOutput(t *testing.T) {
	s := &stub{results: &source.SearchResults{
		Tracks:  []source.Track{{URI: "spotify:track:x1", Name: "Hit", Artist: "Star", Duration: 125 * time.Second}},
		Artists: []source.SearchArtist{{ID: "a1", Name: "Star"}},
		Albums:  []source.SearchAlbum{{ID: "al1", Name: "LP", Artist: "Star"}},
	}}
	_, out, _ := run(t, s, "search", "hit")
	for _, want := range []string{"spotify:track:x1\tHit — Star\t2:05", "spotify:artist:a1\tStar", "spotify:album:al1\tLP — Star"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	_, out, _ = run(t, s, "search", "--json", "hit")
	var js map[string]any
	if err := json.Unmarshal([]byte(out), &js); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	_, out, _ = run(t, &stub{results: &source.SearchResults{}}, "search", "zzz")
	if out != "no results\n" {
		t.Errorf("got %q", out)
	}
	if code, _, _ := run(t, s, "search"); code != ExitUsage {
		t.Error("search without a query is a usage error")
	}
}

func TestDevices(t *testing.T) {
	s := &stub{devices: []source.Device{
		{ID: "d1", Name: "Phone", Type: "Smartphone"},
		{ID: "d2", Name: "Laptop", Type: "Computer", IsActive: true},
	}}
	_, out, _ := run(t, s, "devices")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "* Laptop") || !strings.HasPrefix(lines[1], "  Phone") {
		t.Fatalf("active device should be listed first with *: %q", out)
	}
	_, out, _ = run(t, &stub{}, "devices")
	if !strings.Contains(out, "no devices") {
		t.Errorf("got %q", out)
	}
	_, out, _ = run(t, &stub{}, "devices", "--json")
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty --json should be [], got %q", out)
	}
}

func TestDeviceTransfer(t *testing.T) {
	devs := []source.Device{{ID: "d1", Name: "Living Room Speaker"}, {ID: "d2", Name: "Kitchen Speaker"}, {ID: "d3", Name: "Phone"}}
	s := &stub{devices: devs}
	_, out, _ := run(t, s, "device", "phone")
	if s.calls[0] != "transfer d3" || !strings.Contains(out, "Phone") {
		t.Errorf("exact name (case-insensitive): calls=%v out=%q", s.calls, out)
	}
	s = &stub{devices: devs}
	run(t, s, "device", "kitchen")
	if s.calls[0] != "transfer d2" {
		t.Errorf("substring match: calls=%v", s.calls)
	}
	s = &stub{devices: devs}
	if code, _, e := run(t, s, "device", "speaker"); code != ExitError || !strings.Contains(e, "several") {
		t.Errorf("ambiguous substring: code=%d err=%q", code, e)
	}
	s = &stub{devices: devs}
	if code, _, e := run(t, s, "device", "toaster"); code != ExitError || !strings.Contains(e, "no device matching") {
		t.Errorf("no match: code=%d err=%q", code, e)
	}
	s = &stub{devices: devs}
	run(t, s, "device", "d1")
	if s.calls[0] != "transfer d1" {
		t.Errorf("by id: calls=%v", s.calls)
	}
}

func TestNoDeviceRecoverySingleDevice(t *testing.T) {
	s := &stub{state: playing(), noDev: true, devices: []source.Device{{ID: "d1", Name: "Laptop"}}}
	code, _, e := run(t, s, "next")
	if code != ExitOK {
		t.Fatalf("code=%d err=%q", code, e)
	}
	if strings.Join(s.calls, ",") != "next,transfer d1,next" {
		t.Errorf("calls=%v", s.calls)
	}
	if !strings.Contains(e, "activated Laptop") {
		t.Errorf("stderr=%q", e)
	}
}

func TestNoDeviceRecoveryNoneAndSeveral(t *testing.T) {
	s := &stub{state: playing(), noDev: true}
	code, _, e := run(t, s, "next")
	if code != ExitError || !strings.Contains(e, "open Spotify on any device") {
		t.Errorf("none: code=%d err=%q", code, e)
	}
	s = &stub{state: playing(), noDev: true, devices: []source.Device{{ID: "d1", Name: "A"}, {ID: "d2", Name: "B"}}}
	code, _, e = run(t, s, "next")
	if code != ExitError || !strings.Contains(e, "several available (A, B)") || !strings.Contains(e, "waxon device") {
		t.Errorf("several: code=%d err=%q", code, e)
	}
	if len(s.calls) != 1 {
		t.Errorf("must not transfer when several devices: calls=%v", s.calls)
	}
}

func TestPremiumRequiredMessage(t *testing.T) {
	s := &stub{stateFn: func() (*source.PlaybackState, error) { return nil, source.ErrPremiumRequired }}
	code, _, e := run(t, s, "toggle")
	if code != ExitError || !strings.Contains(e, "Premium") {
		t.Errorf("code=%d err=%q", code, e)
	}
}

func TestUnknownAndNoCommand(t *testing.T) {
	if code, _, e := run(t, &stub{}, "dance"); code != ExitUsage || !strings.Contains(e, "unknown command") {
		t.Errorf("code=%d err=%q", code, e)
	}
	if code, _, _ := run(t, &stub{}); code != ExitUsage {
		t.Error("no args should be a usage error")
	}
}

func TestIsCommand(t *testing.T) {
	for _, c := range Commands {
		if !IsCommand(c) {
			t.Errorf("%s should be a command", c)
		}
	}
	for _, c := range []string{"auth", "demo", "version", "help", ""} {
		if IsCommand(c) {
			t.Errorf("%q must not be a CLI command", c)
		}
	}
}

func TestUsageMentionsEveryCommand(t *testing.T) {
	u := Usage()
	for _, c := range Commands {
		if !strings.Contains(u, "waxon "+c) && !strings.Contains(u, "| "+c) && !strings.Contains(u, c+" |") {
			t.Errorf("usage does not mention %q", c)
		}
	}
}

func TestParseDuration(t *testing.T) {
	for _, c := range []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"0", 0, true},
		{"75", 75 * time.Second, true},
		{"1:15", 75 * time.Second, true},
		{"1:00:01", 3601 * time.Second, true},
		{"1:2:3:4", 0, false},
		{"-5", 0, false},
		{"a", 0, false},
	} {
		got, err := parseDuration(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("parseDuration(%q) = %v, %v", c.in, got, err)
		}
	}
}

func TestErrorsFromSource(t *testing.T) {
	s := &stub{stateFn: func() (*source.PlaybackState, error) { return nil, errors.New("boom") }}
	code, _, e := run(t, s, "status")
	if code != ExitError || !strings.Contains(e, "boom") {
		t.Errorf("code=%d err=%q", code, e)
	}
}
