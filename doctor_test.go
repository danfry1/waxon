package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/config"
	"github.com/danfry1/waxon/player"
	"github.com/danfry1/waxon/source"
	myspotify "github.com/danfry1/waxon/spotify"
)

func healthyEnv(t *testing.T) doctorEnv {
	t.Helper()
	dir := t.TempDir()
	token := filepath.Join(dir, "token.json")
	if err := os.WriteFile(token, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	return doctorEnv{
		configPath: filepath.Join(dir, "config.json"),
		config:     func() (config.Config, error) { return config.Config{ClientID: "bbce036082e24e78ad3905c86ce5946b"}, nil },
		tokenPath:  token,
		profile:    func(context.Context) (string, error) { return "dan", nil },
		devices:    func(context.Context) ([]source.Device, error) { return []source.Device{{Name: "Phone"}}, nil },
		lyrics:     func(context.Context) error { return nil },
		player:     func(string) (string, string, error) { return "", "", player.ErrNotInstalled },
		playerDir:  filepath.Join(dir, "librespot"),
		colour:     "true colour",
		cacheDir:   dir,
		now:        time.Now,
	}
}

func doctorOutput(t *testing.T, env doctorEnv) (string, int) {
	t.Helper()
	var b bytes.Buffer
	failed := printDoctor(&b, runDoctor(context.Background(), env))
	return b.String(), failed
}

func TestDoctorAllGood(t *testing.T) {
	out, failed := doctorOutput(t, healthyEnv(t))
	if failed != 0 || !strings.Contains(out, "All good.") || !strings.Contains(out, "signed in as dan") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
	if !strings.Contains(out, "waxon player login sets it up") {
		t.Errorf("an unused player should say how to set it up:\n%s", out)
	}
}

func TestDoctorNoLogin(t *testing.T) {
	env := healthyEnv(t)
	env.tokenPath = filepath.Join(t.TempDir(), "missing.json")
	out, failed := doctorOutput(t, env)
	if failed == 0 || !strings.Contains(out, "run waxon (or waxon auth) to sign in") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
	if strings.Contains(out, "Devices") {
		t.Error("devices can't be checked without a login")
	}
}

func TestDoctorRejectedLogin(t *testing.T) {
	env := healthyEnv(t)
	env.profile = func(context.Context) (string, error) {
		return "", &myspotify.APIError{StatusCode: 401, Path: "/me"}
	}
	out, failed := doctorOutput(t, env)
	if failed == 0 || !strings.Contains(out, "waxon auth signs in again") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
}

func TestDoctorSharedAppAndNoDevicesWarn(t *testing.T) {
	env := healthyEnv(t)
	env.config = func() (config.Config, error) { return config.Config{ClientID: myauth.DefaultClientID}, nil }
	env.devices = func(context.Context) ([]source.Device, error) { return nil, nil }
	out, failed := doctorOutput(t, env)
	if failed != 0 || !strings.Contains(out, "waxon auth --own") || !strings.Contains(out, "waxon player login") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
	if !strings.Contains(out, "worth a look") {
		t.Errorf("warnings should be summarised:\n%s", out)
	}
}

func TestDoctorPlayerOnButMissing(t *testing.T) {
	env := healthyEnv(t)
	env.config = func() (config.Config, error) { return config.Config{ClientID: "x", Player: "librespot"}, nil }
	out, failed := doctorOutput(t, env)
	if failed == 0 || !strings.Contains(out, "librespot isn't installed") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
}

func TestDoctorOpenLoginFile(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("no Unix permission bits")
	}
	env := healthyEnv(t)
	if err := os.Chmod(env.tokenPath, 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := doctorOutput(t, env)
	if !strings.Contains(out, "chmod 600") {
		t.Errorf("a world-readable login should be flagged:\n%s", out)
	}
}

func TestDoctorUnreadableLogin(t *testing.T) {
	env := healthyEnv(t)
	env.profile = func(context.Context) (string, error) {
		return "", fmt.Errorf("%w: %w", errBadLogin, errors.New("unexpected end of JSON input"))
	}
	out, failed := doctorOutput(t, env)
	if failed == 0 || !strings.Contains(out, "can't be read") || strings.Contains(out, "internet connection") {
		t.Errorf("a corrupt token file should point at waxon auth, not the network:\n%s", out)
	}
}

func TestDoctorBrokenLibrespot(t *testing.T) {
	env := healthyEnv(t)
	env.config = func() (config.Config, error) { return config.Config{ClientID: "x", Player: "librespot"}, nil }
	env.player = func(string) (string, string, error) {
		return "/bin/librespot", "", errors.New("/bin/librespot doesn't run (exec format error)")
	}
	out, failed := doctorOutput(t, env)
	if failed == 0 || !strings.Contains(out, "doesn't run") || !strings.Contains(out, "reinstall") {
		t.Errorf("failed=%d\n%s", failed, out)
	}
}
