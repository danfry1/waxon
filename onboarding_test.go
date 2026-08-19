package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	myauth "github.com/danfry1/waxon/auth"
)

const personalID = "bbce036082e24e78ad3905c86ce5946b"

func newTestOnboarding(input string, interactive bool) (onboarding, *bytes.Buffer, *[]string) {
	var out bytes.Buffer
	var opened []string
	o := onboarding{
		in:          bufio.NewReader(strings.NewReader(input)),
		out:         &out,
		interactive: interactive,
		openBrowser: func(u string) error { opened = append(opened, u); return nil },
	}
	return o, &out, &opened
}

func TestChooseClientIDPrecedence(t *testing.T) {
	o, _, _ := newTestOnboarding("", true)
	// --client-id wins over everything.
	if id, err := o.chooseClientID(authOptions{clientID: personalID}, "envid", "cfgid"); err != nil || id != personalID {
		t.Errorf("flag: %q %v", id, err)
	}
	// --shared forces the shared ID.
	if id, _ := o.chooseClientID(authOptions{shared: true}, personalID, personalID); id != myauth.DefaultClientID {
		t.Errorf("--shared: %q", id)
	}
	// Env var next.
	if id, err := o.chooseClientID(authOptions{}, personalID, ""); err != nil || id != personalID {
		t.Errorf("env: %q %v", id, err)
	}
	// Configured personal ID is reused without prompting.
	o, out, _ := newTestOnboarding("", true)
	if id, err := o.chooseClientID(authOptions{}, "", personalID); err != nil || id != personalID {
		t.Errorf("configured: %q %v", id, err)
	}
	if !strings.Contains(out.String(), "Using your Spotify app") {
		t.Error("should say it's reusing the configured app")
	}
	// A configured *shared* ID still gets the chooser.
	o, out, _ = newTestOnboarding("2\n", true)
	if id, _ := o.chooseClientID(authOptions{}, "", myauth.DefaultClientID); id != myauth.DefaultClientID {
		t.Errorf("chooser with shared configured: %q", id)
	}
	if !strings.Contains(out.String(), "Choose [1/2]") {
		t.Error("expected the chooser")
	}
}

func TestChooserPersonalAppGuide(t *testing.T) {
	// Default choice (Enter) → guide; first paste invalid, second valid.
	o, out, opened := newTestOnboarding("\nnot-an-id\n"+strings.ToUpper(personalID)+"\n", true)
	id, err := o.chooseClientID(authOptions{}, "", "")
	if err != nil || id != personalID {
		t.Fatalf("got %q %v", id, err)
	}
	text := out.String()
	for _, want := range []string{DashboardURL, "http://127.0.0.1:27228/callback", "Create app", "Web API", "Client ID", "32 characters"} {
		if !strings.Contains(text, want) {
			t.Errorf("guide missing %q", want)
		}
	}
	if len(*opened) != 1 || (*opened)[0] != DashboardURL {
		t.Errorf("should open the dashboard once, opened=%v", *opened)
	}
}

func TestChooserSharedChoice(t *testing.T) {
	o, _, opened := newTestOnboarding("2\n", true)
	id, err := o.chooseClientID(authOptions{}, "", "")
	if err != nil || id != myauth.DefaultClientID {
		t.Fatalf("got %q %v", id, err)
	}
	if len(*opened) != 0 {
		t.Error("shared choice must not open a browser")
	}
}

func TestNonInteractiveDefaultsToShared(t *testing.T) {
	o, out, _ := newTestOnboarding("", false)
	id, err := o.chooseClientID(authOptions{}, "", "")
	if err != nil || id != myauth.DefaultClientID {
		t.Fatalf("got %q %v", id, err)
	}
	if !strings.Contains(out.String(), "--client-id") {
		t.Error("should tell non-interactive users how to pass their own ID")
	}
	// --own without a terminal can't read the ID: clear error.
	if _, err := o.chooseClientID(authOptions{own: true}, "", ""); err == nil {
		t.Error("--own non-interactive should error")
	}
}

func TestInputClosedIsAnError(t *testing.T) {
	o, _, _ := newTestOnboarding("", true)
	if _, err := o.chooseClientID(authOptions{}, "", ""); err == nil {
		t.Error("EOF on the chooser should be an error, not a hang or a default")
	}
}

func TestNormalizeClientID(t *testing.T) {
	if id, err := normalizeClientID("  " + strings.ToUpper(personalID) + "\n"); err != nil || id != personalID {
		t.Errorf("%q %v", id, err)
	}
	for _, bad := range []string{"", "abc", strings.Repeat("z", 32), personalID + "0"} {
		if _, err := normalizeClientID(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestParseAuthFlags(t *testing.T) {
	if o, err := parseAuthFlags([]string{"--client-id", personalID}); err != nil || o.clientID != personalID {
		t.Errorf("%+v %v", o, err)
	}
	if o, err := parseAuthFlags([]string{"--own"}); err != nil || !o.own {
		t.Errorf("%+v %v", o, err)
	}
	if _, err := parseAuthFlags([]string{"--shared", "--own"}); err == nil {
		t.Error("conflicting flags should error")
	}
	if _, err := parseAuthFlags([]string{"--bogus"}); err == nil {
		t.Error("unknown flag should error")
	}
}
