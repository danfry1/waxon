package lyrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danfry1/waxon/source"
)

func TestParseLRC(t *testing.T) {
	in := "[00:01.00] First\n[00:03.50] Second\n[ar:Some Artist]\n[00:02.00] Middle\n"
	lines := ParseLRC(in)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (metadata tag should be skipped): %+v", len(lines), lines)
	}
	// Lines must be sorted by timestamp regardless of input order.
	want := []source.LyricLine{
		{Time: 1 * time.Second, Text: "First"},
		{Time: 2 * time.Second, Text: "Middle"},
		{Time: 3*time.Second + 500*time.Millisecond, Text: "Second"},
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %+v, want %+v", i, lines[i], w)
		}
	}
}

func TestParseLRCFractionDigits(t *testing.T) {
	cases := map[string]time.Duration{
		"[00:10.1]":   10*time.Second + 100*time.Millisecond, // tenths
		"[00:10.12]":  10*time.Second + 120*time.Millisecond, // hundredths
		"[00:10.125]": 10*time.Second + 125*time.Millisecond, // thousandths
		"[01:00.00]":  60 * time.Second,                      // minutes carry
	}
	for tag, want := range cases {
		lines := ParseLRC(tag + " x")
		if len(lines) != 1 {
			t.Fatalf("%s: got %d lines, want 1", tag, len(lines))
		}
		if lines[0].Time != want {
			t.Errorf("%s: time = %v, want %v", tag, lines[0].Time, want)
		}
	}
}

func TestParseLRCMultipleTimestamps(t *testing.T) {
	lines := ParseLRC("[00:01.00][00:05.00] Repeat")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (one per timestamp)", len(lines))
	}
	if lines[0].Time != 1*time.Second || lines[1].Time != 5*time.Second {
		t.Errorf("times = %v, %v, want 1s, 5s", lines[0].Time, lines[1].Time)
	}
	if lines[0].Text != "Repeat" || lines[1].Text != "Repeat" {
		t.Errorf("both lines should share the text, got %q / %q", lines[0].Text, lines[1].Text)
	}
}

func TestParseLRCEmpty(t *testing.T) {
	if l := ParseLRC(""); l != nil {
		t.Errorf("ParseLRC(\"\") = %+v, want nil", l)
	}
	if l := ParseLRC("no timestamps here"); l != nil {
		t.Errorf("ParseLRC with no timestamps = %+v, want nil", l)
	}
}

func TestFromResponse(t *testing.T) {
	t.Run("instrumental", func(t *testing.T) {
		got := fromResponse(lrclibResponse{Instrumental: true})
		if got == nil || len(got.Lines) != 1 {
			t.Fatalf("instrumental should yield one line, got %+v", got)
		}
		if got.Synced {
			t.Error("instrumental should not be marked synced")
		}
	})
	t.Run("synced preferred over plain", func(t *testing.T) {
		got := fromResponse(lrclibResponse{
			SyncedLyrics: "[00:01.00] Hi",
			PlainLyrics:  "Hi\nthere",
		})
		if got == nil || !got.Synced {
			t.Fatalf("expected synced lyrics, got %+v", got)
		}
		if len(got.Lines) != 1 || got.Lines[0].Time != time.Second {
			t.Errorf("synced lines wrong: %+v", got.Lines)
		}
	})
	t.Run("plain fallback", func(t *testing.T) {
		got := fromResponse(lrclibResponse{PlainLyrics: "one\ntwo\n"})
		if got == nil || got.Synced {
			t.Fatalf("expected unsynced plain lyrics, got %+v", got)
		}
		if len(got.Lines) != 2 || got.Lines[1].Text != "two" {
			t.Errorf("plain lines wrong: %+v", got.Lines)
		}
	})
	t.Run("none", func(t *testing.T) {
		if got := fromResponse(lrclibResponse{}); got != nil {
			t.Errorf("empty response should yield nil, got %+v", got)
		}
	})
}

func TestClientGet(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if got := r.URL.Query().Get("track_name"); got != "Song" {
			t.Errorf("track_name = %q, want Song", got)
		}
		if got := r.URL.Query().Get("duration"); got != "180" {
			t.Errorf("duration = %q, want 180", got)
		}
		_ = json.NewEncoder(w).Encode(lrclibResponse{SyncedLyrics: "[00:02.00] Hello"})
	}))
	defer srv.Close()

	c := New()
	c.baseURL = srv.URL
	track := source.Track{ID: "t1", Name: "Song", Artist: "Band", Duration: 180 * time.Second}

	got, err := c.Get(context.Background(), track)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || !got.Synced || len(got.Lines) != 1 || got.Lines[0].Text != "Hello" {
		t.Fatalf("unexpected lyrics: %+v", got)
	}

	// Second call for the same track is served from cache (no extra HTTP hit).
	if _, err := c.Get(context.Background(), track); err != nil {
		t.Fatalf("Get (cached): %v", err)
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1 (second call should be cached)", hits)
	}
}

func TestClientGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New()
	c.baseURL = srv.URL
	got, err := c.Get(context.Background(), source.Track{ID: "x", Name: "n", Artist: "a"})
	if err != nil {
		t.Fatalf("404 should not be an error, got %v", err)
	}
	if got != nil {
		t.Errorf("404 should yield nil lyrics, got %+v", got)
	}
}
