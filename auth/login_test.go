package auth

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newTestLogin() *Login {
	return &Login{state: "st", codeCh: make(chan string, 1), errCh: make(chan error, 1)}
}

func TestLoginPasteChecksAddress(t *testing.T) {
	l := newTestLogin()
	for _, bad := range []string{"hello", "http://127.0.0.1/login?code=c&state=other", "http://127.0.0.1/login?state=st"} {
		if err := l.Paste(bad); err == nil {
			t.Errorf("Paste(%q) accepted", bad)
		}
	}
	if err := l.Paste("  127.0.0.1:8888/callback?code=abc&state=st\n"); err != nil {
		t.Fatalf("good paste rejected: %v", err)
	}
	if code := <-l.codeCh; code != "abc" {
		t.Errorf("code = %q", code)
	}
}

func TestLoginRefusedOnSpotifyEndsWait(t *testing.T) {
	l := newTestLogin()
	if err := l.Paste("http://127.0.0.1/login?error=access_denied&state=st"); err == nil {
		t.Fatal("a refusal should be reported")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := l.Wait(ctx); err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Wait = %v, want the refusal", err)
	}
}

func TestLoginWaitCancelled(t *testing.T) {
	l := newTestLogin()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Wait(ctx); err == nil {
		t.Error("a cancelled Wait should fail")
	}
	l.Close() // safe after Wait closed it
}
