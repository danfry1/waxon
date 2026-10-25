package main

import (
	"context"
	"strings"
	"testing"

	"github.com/danfry1/waxon/config"
)

func TestParseScutilProxy(t *testing.T) {
	out := `<dictionary> {
  ExceptionsList : <array> {
    0 : *.local
    1 : 169.254/16
    2 : 10.0.0.0/8
    3 : intranet.corp
  }
  HTTPEnable : 1
  HTTPPort : 3128
  HTTPProxy : web.corp
  HTTPSEnable : 1
  HTTPSPort : 8080
  HTTPSProxy : proxy.corp
}`
	p := parseScutilProxy(out)
	if p.https != "proxy.corp:8080" {
		t.Errorf("https = %q, want proxy.corp:8080", p.https)
	}
	if got := strings.Join(p.noProxy, ","); got != ".local,10.0.0.0/8,intranet.corp" {
		t.Errorf("noProxy = %q (169.254/16 can't be said in NO_PROXY)", got)
	}
	if p.pac != "" {
		t.Errorf("pac = %q, want none", p.pac)
	}
	none := parseScutilProxy("<dictionary> {\n  ExceptionsList : <array> {\n    0 : *.local\n  }\n  FTPPassive : 1\n}")
	if none.https != "" {
		t.Errorf("no proxy configured, got %q", none.https)
	}
	pac := parseScutilProxy("<dictionary> {\n  ProxyAutoConfigEnable : 1\n  ProxyAutoConfigURLString : http://wpad/proxy.pac\n}")
	if pac.pac != "http://wpad/proxy.pac" {
		t.Errorf("pac = %q", pac.pac)
	}
}

// The environment's proxy wins over macOS's settings.
func TestEnvProxyWins(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://mine:1")
	useSystemProxy()
	if got := proxyFromEnv(); got != "http://mine:1" {
		t.Errorf("HTTPS_PROXY = %q, want it left alone", got)
	}
}

// doctor says when a PAC script can't be followed, and how lyrics stand.
func TestDoctorProxyAndLyricsOff(t *testing.T) {
	env := healthyEnv(t)
	env.proxy = func() (string, string) { return "", "http://wpad/proxy.pac" }
	off := false
	env.config = func() (config.Config, error) {
		return config.Config{ClientID: "bbce036082e24e78ad3905c86ce5946b", Lyrics: &off}, nil
	}
	pinged := false
	env.lyrics = func(context.Context) error { pinged = true; return nil }
	out, _ := doctorOutput(t, env)
	if !strings.Contains(out, "PAC script") || !strings.Contains(out, "HTTPS_PROXY") {
		t.Errorf("PAC proxy not explained:\n%s", out)
	}
	if !strings.Contains(out, "off: waxon contacts only Spotify") || pinged {
		t.Errorf("lyrics off: pinged lrclib %v\n%s", pinged, out)
	}
}
