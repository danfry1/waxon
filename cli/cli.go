// Package cli implements waxon's non-interactive subcommands (status, play,
// pause, next, vol, …) so playback can be driven from scripts, keybindings
// and status bars (tmux, waybar, polybar, starship) without opening the TUI.
//
// Every command talks to Spotify through a source.RichSource, the same
// interface the TUI uses, so the commands are testable against a stub and run
// unchanged against demo data.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/danfry1/waxon/source"
)

// Exit codes. Kept distinct so scripts can tell "nothing to do" from failure.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Commands lists every subcommand handled by Run, in help order.
var Commands = []string{
	"status", "play", "pause", "toggle", "next", "prev", "seek",
	"vol", "shuffle", "repeat", "like", "queue", "search", "devices", "device",
}

// IsCommand reports whether name is a subcommand handled by Run.
func IsCommand(name string) bool {
	for _, c := range Commands {
		if c == name {
			return true
		}
	}
	return false
}

// activationSettleDelay mirrors the TUI: give Spotify a moment after a
// transfer before replaying the command on the newly active device.
const activationSettleDelay = 400 * time.Millisecond

// Runner executes subcommands against a source.
type Runner struct {
	Src source.RichSource
	Out io.Writer
	Err io.Writer
	// Sleep is overridable for tests.
	Sleep func(time.Duration)
}

// Run executes args[0] as a subcommand with the remaining args and returns an
// exit code. Errors are written to r.Err.
func (r *Runner) Run(ctx context.Context, args []string) int {
	if r.Sleep == nil {
		r.Sleep = time.Sleep
	}
	if len(args) == 0 {
		fmt.Fprintln(r.Err, Usage())
		return ExitUsage
	}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "status":
		err = r.status(ctx, rest)
	case "play":
		err = r.play(ctx, rest)
	case "pause":
		err = r.withDevice(ctx, func() error { return r.Src.Pause(ctx) })
	case "toggle":
		err = r.toggle(ctx)
	case "next":
		err = r.withDevice(ctx, func() error { return r.Src.Next(ctx) })
	case "prev":
		err = r.withDevice(ctx, func() error { return r.Src.Previous(ctx) })
	case "seek":
		err = r.seek(ctx, rest)
	case "vol":
		err = r.vol(ctx, rest)
	case "shuffle":
		err = r.shuffle(ctx, rest)
	case "repeat":
		err = r.repeat(ctx, rest)
	case "like":
		err = r.like(ctx, rest)
	case "queue":
		err = r.queue(ctx, rest)
	case "search":
		err = r.search(ctx, rest)
	case "devices":
		err = r.devices(ctx, rest)
	case "device":
		err = r.device(ctx, rest)
	default:
		fmt.Fprintf(r.Err, "unknown command %q\n\n%s\n", cmd, Usage())
		return ExitUsage
	}
	if err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			fmt.Fprintln(r.Err, "usage: waxon "+string(ue))
			return ExitUsage
		}
		fmt.Fprintln(r.Err, "error: "+friendly(err))
		return ExitError
	}
	return ExitOK
}

// usageError signals a bad invocation; its value is the usage line to print.
type usageError string

func (u usageError) Error() string { return "usage: waxon " + string(u) }

// friendly rewrites known playback failures into actionable text.
func friendly(err error) string {
	switch {
	case errors.Is(err, source.ErrPremiumRequired):
		return "Spotify Premium is required for playback control"
	case errors.Is(err, source.ErrNoActiveDevice):
		return "no active Spotify device — open Spotify on any device, or run 'waxon devices' and 'waxon device <name>'"
	case errors.Is(err, source.ErrInsufficientScope):
		return "missing permission — run 'waxon auth' once to grant it"
	case errors.Is(err, source.ErrForbidden):
		return "not available with this Spotify app: Spotify blocks this endpoint for development-mode apps (see README)"
	case isRateLimited(err):
		return "Spotify is rate limiting requests — try again shortly; if it persists, use your own client ID (see README)"
	}
	return err.Error()
}

// isRateLimited reports an HTTP 429 from either API path.
func isRateLimited(err error) bool {
	var hse interface{ HTTPStatus() int }
	if errors.As(err, &hse) && hse.HTTPStatus() == 429 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "rate limit")
}

// withDevice runs fn and, if it fails because no device is active, tries to
// recover the way the TUI does: a single available device is activated and
// the command replayed; none or several are reported with guidance.
func (r *Runner) withDevice(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || !errors.Is(err, source.ErrNoActiveDevice) {
		return err
	}
	devs, derr := r.Src.Devices(ctx)
	if derr != nil {
		return err
	}
	switch len(devs) {
	case 0:
		return err
	case 1:
		if terr := r.Src.TransferPlayback(ctx, devs[0].ID); terr != nil {
			return terr
		}
		fmt.Fprintf(r.Err, "activated %s\n", devs[0].Name)
		r.Sleep(activationSettleDelay)
		return fn()
	default:
		names := make([]string, len(devs))
		for i, d := range devs {
			names[i] = d.Name
		}
		// Not wrapped in ErrNoActiveDevice on purpose: friendly() would replace
		// this more specific guidance with the generic message.
		return fmt.Errorf("no active Spotify device; several available (%s) — pick one with 'waxon device <name>'",
			strings.Join(names, ", "))
	}
}

// ---- status ---------------------------------------------------------------

// DefaultStatusFormat is the status output when --format is not given.
const DefaultStatusFormat = "{icon} {title} — {artist}"

type statusJSON struct {
	Playing    bool   `json:"playing"`
	Title      string `json:"title,omitempty"`
	Artist     string `json:"artist,omitempty"`
	Album      string `json:"album,omitempty"`
	ID         string `json:"id,omitempty"`
	URI        string `json:"uri,omitempty"`
	PositionMS int64  `json:"position_ms"`
	DurationMS int64  `json:"duration_ms"`
	Progress   int    `json:"progress"` // 0-100
	Device     string `json:"device,omitempty"`
	Volume     int    `json:"volume"`
	Shuffle    bool   `json:"shuffle"`
	Repeat     string `json:"repeat"`
	Liked      *bool  `json:"liked,omitempty"`
}

// waybarJSON is the shape waybar's custom modules expect with return-type
// "json": text is rendered, alt selects a format-icon, class styles via CSS.
type waybarJSON struct {
	Text       string `json:"text"`
	Alt        string `json:"alt"`
	Class      string `json:"class"`
	Tooltip    string `json:"tooltip,omitempty"`
	Percentage int    `json:"percentage"`
}

func (r *Runner) status(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	waybar := fs.Bool("waybar", false, "print waybar custom-module JSON (text/alt/class/tooltip/percentage)")
	format := fs.String("format", DefaultStatusFormat, "output template")
	withLiked := fs.Bool("liked", false, "also look up whether the track is liked (extra request)")
	if err := fs.Parse(args); err != nil || (*asJSON && *waybar) {
		return usageError("status [--json|--waybar] [--format TEMPLATE] [--liked]")
	}

	ps, err := r.Src.CurrentPlayback(ctx)
	if err != nil {
		return err
	}
	if ps == nil || ps.Track == nil {
		switch {
		case *asJSON:
			return json.NewEncoder(r.Out).Encode(statusJSON{Repeat: string(source.RepeatOff)})
		case *waybar:
			return json.NewEncoder(r.Out).Encode(waybarJSON{Class: "idle", Alt: "idle"})
		}
		// Nothing playing: print nothing so status bars stay blank.
		return nil
	}
	t := ps.Track
	var liked *bool
	if *withLiked {
		if l, lerr := r.Src.IsTrackSaved(ctx, t.ID); lerr == nil {
			liked = &l
		}
	}
	if *waybar {
		state := "paused"
		if t.Playing {
			state = "playing"
		}
		text := FormatStatus(*format, ps, liked)
		if *format == DefaultStatusFormat {
			text = t.Name + " — " + t.Artist // waybar supplies its own icon via format-icons
		}
		return json.NewEncoder(r.Out).Encode(waybarJSON{
			Text:       text,
			Alt:        state,
			Class:      state,
			Tooltip:    fmt.Sprintf("%s\n%s — %s\n%s / %s", t.Name, t.Artist, t.Album, fmtDur(t.Position), fmtDur(t.Duration)),
			Percentage: progressPercent(t),
		})
	}
	if *asJSON {
		repeat := ps.RepeatMode
		if repeat == "" {
			repeat = source.RepeatOff
		}
		return json.NewEncoder(r.Out).Encode(statusJSON{
			Playing:    t.Playing,
			Title:      t.Name,
			Artist:     t.Artist,
			Album:      t.Album,
			ID:         t.ID,
			URI:        t.URI,
			PositionMS: t.Position.Milliseconds(),
			DurationMS: t.Duration.Milliseconds(),
			Progress:   progressPercent(t),
			Device:     t.DeviceName,
			Volume:     ps.Volume,
			Shuffle:    ps.ShuffleOn,
			Repeat:     string(repeat),
			Liked:      liked,
		})
	}
	fmt.Fprintln(r.Out, FormatStatus(*format, ps, liked))
	return nil
}

func progressPercent(t *source.Track) int {
	if t.Duration <= 0 {
		return 0
	}
	p := int(100 * t.Position / t.Duration)
	return max(0, min(100, p))
}

// FormatStatus expands a status template. Placeholders:
//
//	{title} {artist} {album} {position} {duration} {progress} {state} {icon}
//	{device} {volume} {shuffle} {repeat} {liked} {uri} {id}
//
// {state} is "playing"/"paused", {icon} is ▶/⏸, {progress} is 0-100,
// {liked} is ♥ when liked (empty otherwise; requires --liked).
func FormatStatus(format string, ps *source.PlaybackState, liked *bool) string {
	t := ps.Track
	state, icon := "paused", "⏸"
	if t.Playing {
		state, icon = "playing", "▶"
	}
	heart := ""
	if liked != nil && *liked {
		heart = "♥"
	}
	repeat := string(ps.RepeatMode)
	if repeat == "" {
		repeat = string(source.RepeatOff)
	}
	rep := strings.NewReplacer(
		"{title}", t.Name,
		"{artist}", t.Artist,
		"{album}", t.Album,
		"{position}", fmtDur(t.Position),
		"{duration}", fmtDur(t.Duration),
		"{progress}", strconv.Itoa(progressPercent(t)),
		"{state}", state,
		"{icon}", icon,
		"{device}", t.DeviceName,
		"{volume}", strconv.Itoa(ps.Volume),
		"{shuffle}", onOff(ps.ShuffleOn),
		"{repeat}", repeat,
		"{liked}", heart,
		"{uri}", t.URI,
		"{id}", t.ID,
	)
	return rep.Replace(format)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func fmtDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// ---- playback -------------------------------------------------------------

func (r *Runner) play(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return r.withDevice(ctx, func() error { return r.Src.Play(ctx) })
	}
	arg := strings.Join(args, " ")
	if strings.HasPrefix(arg, "spotify:") {
		return r.withDevice(ctx, func() error { return playURI(ctx, r.Src, arg) })
	}
	track, err := r.firstTrack(ctx, arg)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "playing %s — %s\n", track.Name, track.Artist)
	return r.withDevice(ctx, func() error { return playTrack(ctx, r.Src, track) })
}

// playURI starts playback of a Spotify URI: a track plays on its own, any
// other URI (album/playlist/artist) plays as a context.
func playURI(ctx context.Context, src source.RichSource, uri string) error {
	if strings.HasPrefix(uri, "spotify:track:") {
		return src.PlayTrackDirect(ctx, uri)
	}
	return src.PlayTrack(ctx, uri, "")
}

// playTrack plays a track inside its album so playback continues afterwards.
func playTrack(ctx context.Context, src source.RichSource, t source.Track) error {
	if t.AlbumID != "" {
		return src.PlayTrack(ctx, "spotify:album:"+t.AlbumID, t.URI)
	}
	return src.PlayTrackDirect(ctx, t.URI)
}

func (r *Runner) firstTrack(ctx context.Context, query string) (source.Track, error) {
	res, err := r.Src.Search(ctx, query)
	if err != nil {
		return source.Track{}, err
	}
	if res == nil || len(res.Tracks) == 0 {
		return source.Track{}, fmt.Errorf("no tracks found for %q", query)
	}
	return res.Tracks[0], nil
}

func (r *Runner) toggle(ctx context.Context) error {
	ps, err := r.Src.CurrentPlayback(ctx)
	if err != nil {
		return err
	}
	if ps != nil && ps.Track != nil && ps.Track.Playing {
		return r.withDevice(ctx, func() error { return r.Src.Pause(ctx) })
	}
	return r.withDevice(ctx, func() error { return r.Src.Play(ctx) })
}

func (r *Runner) seek(ctx context.Context, args []string) error {
	const usage = "seek <+N|-N|SECONDS|M:SS>"
	if len(args) != 1 {
		return usageError(usage)
	}
	arg := args[0]
	relative := strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-")
	d, err := parseDuration(strings.TrimLeft(arg, "+-"))
	if err != nil {
		return usageError(usage)
	}
	target := d
	if relative {
		ps, perr := r.Src.CurrentPlayback(ctx)
		if perr != nil {
			return perr
		}
		if ps == nil || ps.Track == nil {
			return errors.New("nothing is playing")
		}
		if strings.HasPrefix(arg, "-") {
			d = -d
		}
		target = ps.Track.Position + d
		if target < 0 {
			target = 0
		}
		if ps.Track.Duration > 0 && target > ps.Track.Duration {
			target = ps.Track.Duration
		}
	}
	return r.withDevice(ctx, func() error { return r.Src.Seek(ctx, target) })
}

// parseDuration accepts "90", "1:30" or "1:02:03".
func parseDuration(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, errors.New("bad duration")
	}
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, errors.New("bad duration")
		}
		total = total*60 + n
	}
	return time.Duration(total) * time.Second, nil
}

func (r *Runner) vol(ctx context.Context, args []string) error {
	const usage = "vol <0-100|+N|-N>"
	if len(args) != 1 {
		return usageError(usage)
	}
	arg := args[0]
	n, err := strconv.Atoi(strings.TrimPrefix(arg, "+"))
	if err != nil {
		return usageError(usage)
	}
	if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
		ps, perr := r.Src.CurrentPlayback(ctx)
		if perr != nil {
			return perr
		}
		cur := 0
		if ps != nil {
			cur = ps.Volume
		}
		n = cur + n
	}
	n = max(0, min(100, n))
	if err := r.withDevice(ctx, func() error { return r.Src.SetVolume(ctx, n) }); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "volume %d%%\n", n)
	return nil
}

func (r *Runner) shuffle(ctx context.Context, args []string) error {
	const usage = "shuffle [on|off]"
	var want bool
	switch {
	case len(args) == 0:
		ps, err := r.Src.CurrentPlayback(ctx)
		if err != nil {
			return err
		}
		want = ps == nil || !ps.ShuffleOn
	case len(args) == 1 && (args[0] == "on" || args[0] == "off"):
		want = args[0] == "on"
	default:
		return usageError(usage)
	}
	if err := r.withDevice(ctx, func() error { return r.Src.SetShuffle(ctx, want) }); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "shuffle %s\n", onOff(want))
	return nil
}

func (r *Runner) repeat(ctx context.Context, args []string) error {
	const usage = "repeat <off|all|one>"
	if len(args) != 1 {
		return usageError(usage)
	}
	var mode source.RepeatMode
	switch args[0] {
	case "off":
		mode = source.RepeatOff
	case "all", "context":
		mode = source.RepeatContext
	case "one", "track":
		mode = source.RepeatTrack
	default:
		return usageError(usage)
	}
	if err := r.withDevice(ctx, func() error { return r.Src.SetRepeat(ctx, mode) }); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "repeat %s\n", args[0])
	return nil
}

// ---- library --------------------------------------------------------------

func (r *Runner) like(ctx context.Context, args []string) error {
	const usage = "like [on|off]"
	ps, err := r.Src.CurrentPlayback(ctx)
	if err != nil {
		return err
	}
	if ps == nil || ps.Track == nil {
		return errors.New("nothing is playing")
	}
	id := ps.Track.ID
	saved, err := r.Src.IsTrackSaved(ctx, id)
	if err != nil {
		return err
	}
	want := !saved
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "on":
		want = true
	case len(args) == 1 && args[0] == "off":
		want = false
	default:
		return usageError(usage)
	}
	if want == saved {
		fmt.Fprintf(r.Out, "%s — %s already %s\n", ps.Track.Name, ps.Track.Artist, likedWord(saved))
		return nil
	}
	if want {
		err = r.Src.SaveTrack(ctx, id)
	} else {
		err = r.Src.RemoveTrack(ctx, id)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "%s — %s %s\n", ps.Track.Name, ps.Track.Artist, likedWord(want))
	return nil
}

func likedWord(liked bool) string {
	if liked {
		return "liked"
	}
	return "unliked"
}

func (r *Runner) queue(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError("queue <spotify:track:URI|search terms>")
	}
	arg := strings.Join(args, " ")
	if strings.HasPrefix(arg, "spotify:track:") {
		id := strings.TrimPrefix(arg, "spotify:track:")
		if err := r.withDevice(ctx, func() error { return r.Src.AddToQueue(ctx, id) }); err != nil {
			return err
		}
		fmt.Fprintf(r.Out, "queued %s\n", arg)
		return nil
	}
	track, err := r.firstTrack(ctx, arg)
	if err != nil {
		return err
	}
	if err := r.withDevice(ctx, func() error { return r.Src.AddToQueue(ctx, track.ID) }); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "queued %s — %s\n", track.Name, track.Artist)
	return nil
}

func (r *Runner) search(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return usageError("search [--json] <query>")
	}
	res, err := r.Src.Search(ctx, strings.Join(fs.Args(), " "))
	if err != nil {
		return err
	}
	if res == nil {
		res = &source.SearchResults{}
	}
	if *asJSON {
		type jTrack struct {
			Name       string `json:"name"`
			Artist     string `json:"artist"`
			Album      string `json:"album"`
			URI        string `json:"uri"`
			ID         string `json:"id"`
			DurationMS int64  `json:"duration_ms"`
		}
		type jArtist struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URI  string `json:"uri"`
		}
		type jAlbum struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Artist string `json:"artist"`
			URI    string `json:"uri"`
		}
		out := struct {
			Tracks  []jTrack  `json:"tracks"`
			Artists []jArtist `json:"artists"`
			Albums  []jAlbum  `json:"albums"`
		}{Tracks: []jTrack{}, Artists: []jArtist{}, Albums: []jAlbum{}} // never null
		for _, t := range res.Tracks {
			out.Tracks = append(out.Tracks, jTrack{t.Name, t.Artist, t.Album, t.URI, t.ID, t.Duration.Milliseconds()})
		}
		for _, a := range res.Artists {
			out.Artists = append(out.Artists, jArtist{a.ID, a.Name, "spotify:artist:" + a.ID})
		}
		for _, a := range res.Albums {
			out.Albums = append(out.Albums, jAlbum{a.ID, a.Name, a.Artist, "spotify:album:" + a.ID})
		}
		return json.NewEncoder(r.Out).Encode(out)
	}
	if len(res.Tracks)+len(res.Artists)+len(res.Albums) == 0 {
		fmt.Fprintln(r.Out, "no results")
		return nil
	}
	for _, t := range res.Tracks {
		fmt.Fprintf(r.Out, "%s\t%s — %s\t%s\n", t.URI, t.Name, t.Artist, fmtDur(t.Duration))
	}
	for _, a := range res.Artists {
		fmt.Fprintf(r.Out, "spotify:artist:%s\t%s\n", a.ID, a.Name)
	}
	for _, a := range res.Albums {
		fmt.Fprintf(r.Out, "spotify:album:%s\t%s — %s\n", a.ID, a.Name, a.Artist)
	}
	return nil
}

// ---- devices --------------------------------------------------------------

func (r *Runner) devices(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("devices", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return usageError("devices [--json]")
	}
	devs, err := r.Src.Devices(ctx)
	if err != nil {
		return err
	}
	if *asJSON {
		type jDevice struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Type   string `json:"type"`
			Active bool   `json:"active"`
		}
		out := make([]jDevice, 0, len(devs)) // never null
		for _, d := range devs {
			out = append(out, jDevice{d.ID, d.Name, d.Type, d.IsActive})
		}
		return json.NewEncoder(r.Out).Encode(out)
	}
	if len(devs) == 0 {
		fmt.Fprintln(r.Out, "no devices — open Spotify on any device")
		return nil
	}
	sort.SliceStable(devs, func(i, j int) bool { return devs[i].IsActive && !devs[j].IsActive })
	for _, d := range devs {
		mark := " "
		if d.IsActive {
			mark = "*"
		}
		fmt.Fprintf(r.Out, "%s %s\t%s\t%s\n", mark, d.Name, d.Type, d.ID)
	}
	return nil
}

func (r *Runner) device(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError("device <name|id>")
	}
	want := strings.ToLower(strings.Join(args, " "))
	devs, err := r.Src.Devices(ctx)
	if err != nil {
		return err
	}
	var match *source.Device
	for i := range devs {
		d := &devs[i]
		if d.ID == want || strings.ToLower(d.Name) == want {
			match = d
			break
		}
	}
	if match == nil {
		for i := range devs {
			d := &devs[i]
			if strings.Contains(strings.ToLower(d.Name), want) {
				if match != nil {
					return fmt.Errorf("%q matches several devices; be more specific", want)
				}
				match = d
			}
		}
	}
	if match == nil {
		return fmt.Errorf("no device matching %q (see 'waxon devices')", want)
	}
	if err := r.Src.TransferPlayback(ctx, match.ID); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "playback → %s\n", match.Name)
	return nil
}

// Usage returns the subcommand help text.
func Usage() string {
	return strings.TrimSpace(`
Playback commands (scriptable, no TUI):
  waxon status [--json|--waybar] [--format T] [--liked]
                                Now playing. Default format: "` + DefaultStatusFormat + `"
                                Placeholders: {title} {artist} {album} {position} {duration}
                                {progress} {state} {icon} {device} {volume} {shuffle}
                                {repeat} {liked} {uri} {id}. Prints nothing when idle.
  waxon play [query|spotify:URI]  Resume, or play the first match / the URI
  waxon pause | toggle | next | prev
  waxon seek <+N|-N|SECS|M:SS>    Seek relative or absolute
  waxon vol <0-100|+N|-N>         Set or nudge volume
  waxon shuffle [on|off]          Toggle or set shuffle
  waxon repeat <off|all|one>
  waxon like [on|off]             Like/unlike the playing track
  waxon queue <query|spotify:track:URI>
  waxon search [--json] <query>
  waxon devices [--json]          List Spotify Connect devices (* = active)
  waxon device <name|id>          Transfer playback to a device`)
}
