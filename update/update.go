// Package update asks GitHub, at most once a day, whether a newer waxon has
// been released. It sends nothing but the request itself: no identifiers,
// no usage.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// latestURL answers with the newest release (never a prerelease or draft).
const latestURL = "https://api.github.com/repos/danfry1/waxon/releases/latest"

// ReleasesURL is where to get a release by hand.
const ReleasesURL = "https://github.com/danfry1/waxon/releases/latest"

// interval is how long an answer is kept before GitHub is asked again.
const interval = 24 * time.Hour

// state is the last answer, kept between launches.
type state struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest,omitempty"`
}

// Available returns the newest release's version (without the "v") when
// it's newer than current, else "". The last answer is kept at path, so
// GitHub is asked at most once a day; a failed check also waits a day.
// Builds without a release version ("dev") never check, and nor does
// anything when there's nowhere to keep the answer (path ""), which would
// otherwise ask on every launch.
func Available(ctx context.Context, current, path string) string {
	return available(ctx, current, path, time.Now(), fetchLatest)
}

// Known is what the last check found, without asking GitHub: a newer
// release's version, or "".
func Known(current, path string) string {
	if latest := load(path).Latest; newer(latest, current) {
		return strings.TrimPrefix(latest, "v")
	}
	return ""
}

func available(ctx context.Context, current, path string, now time.Time,
	fetch func(context.Context) (string, error),
) string {
	if _, ok := parse(current); !ok || path == "" {
		return ""
	}
	s := load(path)
	if now.Sub(s.Checked) >= interval || now.Before(s.Checked) {
		s.Checked = now
		if latest, err := fetch(ctx); err != nil {
			slog.Debug("update check failed", "error", err)
		} else {
			s.Latest = latest
		}
		if err := save(path, s); err != nil {
			slog.Debug("update check not saved", "error", err)
		}
	}
	return Known(current, path)
}

func fetchLatest(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "waxon")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", err
	}
	if _, ok := parse(release.Tag); !ok {
		return "", fmt.Errorf("unexpected release tag %q", release.Tag)
	}
	return release.Tag, nil
}

func load(path string) state {
	var s state
	if path == "" {
		return s
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s) // a damaged file just means checking again
	}
	return s
}

// save writes s atomically, readable by the user alone. The temporary
// file is unique, so two waxons starting at once can't mix their writes.
func save(path string, s state) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "update-*.tmp") // 0o600
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // gone already once renamed
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// parse reads "v1.2.3" or "1.2.3"; anything else (a dev build, a
// prerelease) is not ok.
func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// newer reports whether version a is newer than b.
func newer(a, b string) bool {
	va, okA := parse(a)
	vb, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

// Command is how to upgrade the waxon at exe (the running binary, symlinks
// resolved), judged by where it's installed: "" when that can't be told,
// for a download or a build from source.
func Command(exe string) string {
	p := filepath.ToSlash(exe)
	lower := strings.ToLower(p)
	switch {
	case strings.Contains(p, "/Cellar/"):
		return "brew upgrade waxon"
	case strings.Contains(lower, "/scoop/"):
		return "scoop update waxon"
	case strings.Contains(p, "/nix/store/"):
		return "" // a flake, a profile or a system config: each updates its own way
	case strings.HasSuffix(filepath.ToSlash(filepath.Dir(exe)), "/go/bin") ||
		(os.Getenv("GOBIN") != "" && filepath.Dir(exe) == os.Getenv("GOBIN")):
		return "go install github.com/danfry1/waxon@latest"
	}
	return ""
}
