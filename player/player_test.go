package player

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// The test binary doubles as a fake librespot: with WAXON_FAKE_LIBRESPOT set
// it acts out that scenario instead of running the tests.
func TestMain(m *testing.M) {
	if mode := os.Getenv("WAXON_FAKE_LIBRESPOT"); mode != "" {
		os.Exit(fakeLibrespot(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeLibrespot(mode string, args []string) int {
	dir := ""
	for i, a := range args {
		if a == "--system-cache" && i+1 < len(args) {
			dir = args[i+1]
		}
	}
	switch mode {
	case "login": // the user approves after a moment; then it keeps playing
		fmt.Println("Browse to: https://accounts.spotify.com/authorize?…")
		time.Sleep(200 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(`{}`), 0o644) // as librespot does
		time.Sleep(time.Minute)
	case "abandon": // the user gives up: no credentials
		return 1
	case "serve":
		fmt.Fprintln(os.Stderr, "[INFO  librespot] "+strings.Join(args, " "))
		time.Sleep(time.Minute)
	case "fail":
		fmt.Fprintln(os.Stderr, "[2026-10-24T14:04:09Z ERROR librespot] could not initialize spirc: Bad credentials")
		return 1
	}
	return 0
}

func fake(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("WAXON_FAKE_LIBRESPOT", mode)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

func loggedInDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(credentialsPath(dir), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindMissing(t *testing.T) {
	if _, err := Find(filepath.Join(t.TempDir(), "no-librespot")); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Find = %v, want ErrNotInstalled", err)
	}
}

func TestLoginWaitsForCredentialsThenStops(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "librespot")
	var out bytes.Buffer
	start := time.Now()
	if err := Login(context.Background(), fake(t, "login"), dir, false, nil, &out); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !LoggedIn(dir) {
		t.Error("credentials not cached")
	}
	if time.Since(start) > 10*time.Second {
		t.Error("Login should stop librespot once signed in, not wait for it to exit")
	}
	if !strings.Contains(out.String(), "Browse to") {
		t.Errorf("librespot's sign-in prompt should reach the user, got %q", out.String())
	}
}

func TestLoginAbandoned(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "librespot")
	err := Login(context.Background(), fake(t, "abandon"), dir, false, nil, io.Discard)
	if err == nil || LoggedIn(dir) {
		t.Errorf("Login = %v; an exit without credentials must fail", err)
	}
}

func TestStartNeedsLogin(t *testing.T) {
	if _, err := Start(fake(t, "serve"), t.TempDir(), "waxon", io.Discard, nil); !errors.Is(err, ErrNoLogin) {
		t.Errorf("Start = %v, want ErrNoLogin", err)
	}
}

func TestStartAndStop(t *testing.T) {
	var log syncBuffer
	exited := make(chan error, 1)
	s, err := Start(fake(t, "serve"), loggedInDir(t), "waxon (box)", &log, func(err error) { exited <- err })
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(log.String(), "--disable-discovery") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := log.String(); !strings.Contains(got, "--name waxon (box)") || strings.Contains(got, "--cache ") || !strings.Contains(got, "--bitrate 320") {
		t.Errorf("unexpected librespot args: %q", got)
	}
	s.Stop()
	select {
	case err := <-exited:
		t.Errorf("a requested Stop must not report an exit, got %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	s.Stop() // idempotent
}

func TestUnexpectedExitReportsLibrespotError(t *testing.T) {
	exited := make(chan error, 1)
	_, err := Start(fake(t, "fail"), loggedInDir(t), "waxon", io.Discard, func(err error) { exited <- err })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-exited:
		if err == nil || !strings.Contains(err.Error(), "Bad credentials") || strings.Contains(err.Error(), "2026-") {
			t.Errorf("exit error = %v; want librespot's message without its log prefix", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("unexpected exit not reported")
	}
}

func TestLogoutRemovesLogin(t *testing.T) {
	dir := loggedInDir(t)
	if err := Logout(dir); err != nil || LoggedIn(dir) {
		t.Errorf("Logout = %v, logged in = %v", err, LoggedIn(dir))
	}
	if err := Logout(dir); err != nil {
		t.Errorf("second Logout = %v", err)
	}
}

// syncBuffer is a bytes.Buffer safe for librespot's output goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestFailedReloginKeepsWorkingLogin(t *testing.T) {
	dir := loggedInDir(t)
	if err := Login(context.Background(), fake(t, "abandon"), dir, false, nil, io.Discard); err == nil {
		t.Fatal("abandoned sign-in should fail")
	}
	if !LoggedIn(dir) {
		t.Error("a failed re-login must not remove the working login")
	}
}

func TestReloginSignsInAfresh(t *testing.T) {
	dir := loggedInDir(t)
	if err := os.WriteFile(credentialsPath(dir), []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Login(context.Background(), fake(t, "login"), dir, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(credentialsPath(dir)); string(data) != `{}` {
		t.Errorf("credentials = %s; want the new sign-in's", data)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "login-*")); len(left) != 0 {
		t.Errorf("sign-in dir left behind: %v", left)
	}
}

func TestLoginKeepsCredentialsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" { // no Unix permission bits
		t.Skip()
	}
	dir := filepath.Join(t.TempDir(), "librespot")
	if err := Login(context.Background(), fake(t, "login"), dir, false, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(credentialsPath(dir))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("credentials mode = %v (%v), want 0600", info.Mode().Perm(), err)
	}
	dirInfo, _ := os.Stat(dir)
	if dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, want 0700", dirInfo.Mode().Perm())
	}
}

// The setup passes a pipe for pasting; Login must still return once signed
// in, though nothing is ever written to it (os/exec waits for its stdin copy).
func TestLoginReturnsWithUnusedPipeStdin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "librespot")
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	done := make(chan error, 1)
	go func() { done <- Login(context.Background(), fake(t, "login"), dir, false, pr, io.Discard) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Login hung waiting on its stdin pipe")
	}
}
