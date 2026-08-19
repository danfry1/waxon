package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/config"
	"github.com/mattn/go-isatty"
)

// Onboarding: choosing which Spotify "app" (client ID) waxon talks through.
//
// Spotify rate-limits the bundled shared app as a whole, so the best
// experience is a personal client ID — but creating one is a two-minute
// detour most people don't know they need. `waxon auth` therefore walks
// through the choice and, for a personal app, the exact dashboard steps.

// DashboardURL is Spotify's developer dashboard where apps are created.
const DashboardURL = "https://developer.spotify.com/dashboard"

// authOptions are the non-interactive ways to pick a client ID.
type authOptions struct {
	clientID string // --client-id
	shared   bool   // --shared
	own      bool   // --own: force the personal-app guide even if configured
}

func parseAuthFlags(args []string) (authOptions, error) {
	var o authOptions
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.clientID, "client-id", "", "use this Spotify client ID")
	fs.BoolVar(&o.shared, "shared", false, "use the bundled shared client ID")
	fs.BoolVar(&o.own, "own", false, "set up (or switch to) a personal client ID")
	if err := fs.Parse(args); err != nil {
		return o, errors.New("usage: waxon auth [--client-id ID | --shared | --own]")
	}
	if o.shared && (o.clientID != "" || o.own) {
		return o, errors.New("--shared cannot be combined with --client-id or --own")
	}
	return o, nil
}

// onboarding holds the I/O the flow talks through, so it can be tested.
type onboarding struct {
	in          *bufio.Reader
	out         io.Writer
	interactive bool
	openBrowser func(string) error
}

func newOnboarding() onboarding {
	return onboarding{
		in:          bufio.NewReader(os.Stdin),
		out:         os.Stdout,
		interactive: isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()),
		openBrowser: myauth.OpenBrowser,
	}
}

// chooseClientID decides which client ID to authenticate with.
// Precedence: flags, $SPOTIFY_CLIENT_ID, an already-configured personal ID,
// then the interactive chooser (or the shared ID when not interactive).
func (o onboarding) chooseClientID(opts authOptions, env, configured string) (string, error) {
	switch {
	case opts.clientID != "":
		return normalizeClientID(opts.clientID)
	case opts.shared:
		return myauth.DefaultClientID, nil
	case opts.own:
		return o.guidePersonalApp()
	case env != "":
		return normalizeClientID(env)
	case configured != "" && configured != myauth.DefaultClientID:
		fmt.Fprintf(o.out, "  Using your Spotify app (client ID %s…).\n", configured[:8])
		fmt.Fprintln(o.out, "  (run 'waxon auth --own' or '--shared' to change)")
		return configured, nil
	case !o.interactive:
		fmt.Fprintln(o.out, "  No terminal to ask on; using the shared Spotify app.")
		fmt.Fprintln(o.out, "  For your own: waxon auth --client-id <ID>")
		return myauth.DefaultClientID, nil
	}
	return o.askWhichApp()
}

func (o onboarding) askWhichApp() (string, error) {
	fmt.Fprint(o.out, `
  waxon talks to Spotify through a Spotify "app". Pick one:

    1) Your own app  (recommended, ~2 minutes)
       No rate limits. Spotify blocks one thing for personal apps:
       an artist's "top tracks" (artist pages still show albums).

    2) The shared app  (zero setup)
       Works immediately, but Spotify throttles it for everyone at
       once, so expect occasional "rate limit" pauses.

`)
	for {
		fmt.Fprint(o.out, "  Choose [1/2] (1): ")
		line, err := o.readLine()
		if err != nil {
			return "", err
		}
		switch strings.TrimSpace(line) {
		case "", "1":
			return o.guidePersonalApp()
		case "2":
			return myauth.DefaultClientID, nil
		}
	}
}

// guidePersonalApp walks through creating a Spotify app and returns the
// pasted client ID.
func (o onboarding) guidePersonalApp() (string, error) {
	redirect := myauth.RedirectURI("personal") // any non-shared ID → /callback
	fmt.Fprintf(o.out, `
  Create your Spotify app (it's free):

    1. Open %s and log in.
    2. Click "Create app". Name and description: anything, e.g. "waxon".
    3. Redirect URI — paste exactly, then click Add:

         %s

    4. Under "Which API/SDKs are you planning to use?" tick "Web API".
       Save.
    5. On the app's page click "Settings" and copy the Client ID.

`, DashboardURL, redirect)
	if o.interactive && o.openBrowser != nil {
		if err := o.openBrowser(DashboardURL); err == nil {
			fmt.Fprintln(o.out, "  (opened the dashboard in your browser)")
		}
	}
	if !o.interactive {
		return "", errors.New("no terminal to read the client ID from; run: waxon auth --client-id <ID>")
	}
	for {
		fmt.Fprint(o.out, "  Paste your Client ID: ")
		line, err := o.readLine()
		if err != nil {
			return "", err
		}
		id, err := normalizeClientID(line)
		if err != nil {
			fmt.Fprintf(o.out, "  %v — try again (or Ctrl-C).\n", err)
			continue
		}
		return id, nil
	}
}

func (o onboarding) readLine() (string, error) {
	line, err := o.in.ReadString('\n')
	if err != nil && line == "" {
		if errors.Is(err, io.EOF) {
			return "", errors.New("input closed")
		}
		return "", err
	}
	return line, nil
}

// normalizeClientID trims and validates a pasted client ID: Spotify issues
// 32 lowercase hex characters.
func normalizeClientID(s string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(s))
	if len(id) != 32 {
		return "", fmt.Errorf("a client ID is 32 characters (got %d)", len(id))
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", errors.New("a client ID is hexadecimal (0-9, a-f)")
		}
	}
	return id, nil
}

// runAuthWith performs the full setup: choose a client ID, run the OAuth
// flow in the browser, save the token and client ID.
func runAuthWith(o onboarding, opts authOptions) {
	runAuthFlow(o, opts, false)
}

// runAuthFlow is runAuthWith; launching=true tailors the closing message for
// the first-run path that continues straight into the TUI.
func runAuthFlow(o onboarding, opts authOptions, launching bool) {
	fmt.Fprintln(o.out, "")
	fmt.Fprintln(o.out, "  waxon setup")

	saved, _ := config.Load()
	clientID, err := o.chooseClientID(opts, os.Getenv("SPOTIFY_CLIENT_ID"), saved.ClientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n  %v\n", err)
		os.Exit(2)
	}
	if clientID == myauth.DefaultClientID {
		fmt.Fprintln(o.out, "\n  Using the shared Spotify app. Switch any time with: waxon auth --own")
	}

	fmt.Fprintln(o.out, "\n  Connecting your Spotify account...")
	authenticateWith(clientID)

	fmt.Fprintln(o.out, "")
	if launching {
		fmt.Fprintln(o.out, "  You're all set — starting waxon.")
	} else {
		fmt.Fprintln(o.out, "  You're all set! Run 'waxon' to start.")
	}
	fmt.Fprintln(o.out, "")
}
