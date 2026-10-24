package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// spotifyAppLauncher returns a function that starts the Spotify desktop
// app in the background, or nil when it isn't installed here. The device
// picker offers it when the app isn't online, so switching to it doesn't
// mean leaving the terminal.
func spotifyAppLauncher() func() error {
	home, _ := os.UserHomeDir()
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	switch runtime.GOOS {
	case "darwin":
		for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if exists(filepath.Join(dir, "Spotify.app")) {
				// -g: start it without bringing it to the front.
				return func() error { return exec.Command("open", "-g", "-a", "Spotify").Run() }
			}
		}
	case "linux":
		if path, err := exec.LookPath("spotify"); err == nil {
			return func() error { return startDetached(exec.Command(path)) }
		}
		for _, dir := range []string{"/var/lib/flatpak/app", filepath.Join(home, ".local/share/flatpak/app")} {
			if exists(filepath.Join(dir, "com.spotify.Client")) {
				return func() error { return startDetached(exec.Command("flatpak", "run", "com.spotify.Client")) }
			}
		}
	case "windows":
		for _, exe := range []string{
			filepath.Join(os.Getenv("APPDATA"), "Spotify", "Spotify.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps", "Spotify.exe"), // Microsoft Store
		} {
			if exists(exe) {
				// exe is Spotify's own install path in the user's profile.
				return func() error { return startDetached(exec.Command(exe, "--minimized")) } //nolint:gosec // see above
			}
		}
	}
	return nil
}

// startDetached starts cmd in its own session, so closing waxon's terminal
// doesn't take the app with it, and doesn't wait for it.
func startDetached(cmd *exec.Cmd) error {
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// thisComputerNames are the names the Spotify app on this machine may give
// its device: the host name and, on macOS, the computer name ("Daniel's
// MacBook Pro"), which it uses there.
func thisComputerNames() []string {
	var names []string
	if host, err := os.Hostname(); err == nil && host != "" {
		host, _, _ = strings.Cut(host, ".")
		names = append(names, host)
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("scutil", "--get", "ComputerName").Output(); err == nil {
			if name := strings.TrimSpace(string(out)); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
