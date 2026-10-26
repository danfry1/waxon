package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fetcher answers with latest (or err) and counts the times it was asked.
func fetcher(latest string, err error, calls *int) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		*calls++
		return latest, err
	}
}

func TestAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	calls := 0
	fetch := fetcher("v1.14.0", nil, &calls)

	if got := available(context.Background(), "1.13.0", path, now, fetch); got != "1.14.0" {
		t.Fatalf("available = %q, want 1.14.0", got)
	}
	// Within a day the kept answer is used.
	if got := available(context.Background(), "v1.13.0", path, now.Add(23*time.Hour), fetch); got != "1.14.0" || calls != 1 {
		t.Errorf("a second launch the same day: %q after %d checks, want 1.14.0 after 1", got, calls)
	}
	// A day later GitHub is asked again.
	available(context.Background(), "1.13.0", path, now.Add(25*time.Hour), fetch)
	if calls != 2 {
		t.Errorf("checks after a day = %d, want 2", calls)
	}
	// The newest release: nothing to say.
	if got := available(context.Background(), "1.14.0", path, now.Add(25*time.Hour), fetch); got != "" {
		t.Errorf("on the newest release: %q, want nothing", got)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("kept answer: %v, %v; want a 0600 file", info, err)
	}
}

// A failed check waits a day too, keeping what was known before.
func TestAvailableFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	calls := 0
	available(context.Background(), "1.13.0", path, now, fetcher("v1.14.0", nil, &calls))
	failing := fetcher("", errors.New("offline"), &calls)
	if got := available(context.Background(), "1.13.0", path, now.Add(25*time.Hour), failing); got != "1.14.0" {
		t.Errorf("after a failed check: %q, want the earlier answer", got)
	}
	available(context.Background(), "1.13.0", path, now.Add(26*time.Hour), failing)
	if calls != 2 {
		t.Errorf("checks = %d, want 2 (no retry within a day of a failure)", calls)
	}
}

// A dev build never checks.
func TestAvailableDevBuild(t *testing.T) {
	calls := 0
	if got := available(context.Background(), "dev", "", time.Now(), fetcher("v9.0.0", nil, &calls)); got != "" || calls != 0 {
		t.Errorf("dev build: %q after %d checks, want nothing and none", got, calls)
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"v1.14.0", "1.13.0", true},
		{"v1.13.1", "v1.13.0", true},
		{"v2.0.0", "1.99.9", true},
		{"v1.10.0", "1.9.0", true},
		{"v1.13.0", "1.13.0", false},
		{"v1.12.0", "1.13.0", false},
		{"", "1.13.0", false},
		{"v1.14.0-rc1", "1.13.0", false},
		{"v1.14.0", "dev", false},
	} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCommand(t *testing.T) {
	t.Setenv("GOBIN", "")
	for exe, want := range map[string]string{
		"/opt/homebrew/Cellar/waxon/1.13.0/bin/waxon":              "brew upgrade waxon",
		"/home/linuxbrew/.linuxbrew/Cellar/waxon/1.13.0/bin/waxon": "brew upgrade waxon",
		"C:/Users/me/scoop/apps/waxon/current/waxon.exe":           "scoop update waxon",
		"/nix/store/abc-waxon-1.13.0/bin/waxon":                    "",
		"/Users/me/go/bin/waxon":                                   "go install github.com/danfry1/waxon@latest",
		"/usr/local/bin/waxon":                                     "",
	} {
		if got := Command(exe); got != want {
			t.Errorf("Command(%q) = %q, want %q", exe, got, want)
		}
	}
}

// With nowhere to keep the answer, waxon doesn't check: it would otherwise
// ask on every launch.
func TestAvailableNowhereToKeep(t *testing.T) {
	calls := 0
	if got := available(context.Background(), "1.13.0", "", time.Now(), fetcher("v1.14.0", nil, &calls)); got != "" || calls != 0 {
		t.Errorf("no state path: %q after %d checks, want nothing and none", got, calls)
	}
}

// Known reads the last answer without asking.
func TestKnown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	if got := Known("1.13.0", path); got != "" {
		t.Errorf("never checked: %q, want nothing", got)
	}
	calls := 0
	available(context.Background(), "1.13.0", path, time.Now(), fetcher("v1.14.0", nil, &calls))
	if got := Known("1.13.0", path); got != "1.14.0" {
		t.Errorf("Known = %q, want 1.14.0", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("left behind %d files, want just update.json", len(entries))
	}
}
