package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Go programs find a proxy in HTTPS_PROXY and NO_PROXY only, but on a Mac
// a corporate proxy is usually set in System Settings. useSystemProxy copies
// the HTTPS proxy and its exceptions from there into the environment, so
// waxon reaches Spotify the way other apps do. A PAC script (a proxy chosen
// per address by a script) can't be followed; doctor says so.

// systemProxy is the HTTPS proxy macOS is set to use.
type systemProxy struct {
	https   string   // "host:port", or "" for none
	noProxy []string // hosts and domains to reach directly
	pac     string   // the PAC script's URL, when one is set
}

// parseScutilProxy reads `scutil --proxy` output.
func parseScutilProxy(out string) systemProxy {
	var p systemProxy
	vals := map[string]string{}
	inExceptions := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "ExceptionsList"):
			inExceptions = true
		case inExceptions && line == "}":
			inExceptions = false
		case inExceptions:
			if _, v, ok := strings.Cut(line, " : "); ok {
				if host := noProxyEntry(strings.TrimSpace(v)); host != "" {
					p.noProxy = append(p.noProxy, host)
				}
			}
		default:
			if k, v, ok := strings.Cut(line, " : "); ok {
				vals[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	if vals["HTTPSEnable"] == "1" && vals["HTTPSProxy"] != "" {
		p.https = vals["HTTPSProxy"]
		if port := vals["HTTPSPort"]; port != "" {
			p.https += ":" + port
		}
	}
	if vals["ProxyAutoConfigEnable"] == "1" {
		p.pac = vals["ProxyAutoConfigURLString"]
	}
	return p
}

// noProxyEntry turns a macOS exception ("*.example.com", "10.0.0.0/8") into
// a NO_PROXY entry, or "" for the abbreviated network forms ("169.254/16")
// NO_PROXY can't express.
func noProxyEntry(v string) string {
	if strings.Contains(v, "/") && strings.Count(strings.SplitN(v, "/", 2)[0], ".") != 3 {
		return ""
	}
	return strings.TrimPrefix(v, "*")
}

// readSystemProxy asks macOS for its proxy settings (nothing elsewhere).
func readSystemProxy() systemProxy {
	if runtime.GOOS != "darwin" {
		return systemProxy{}
	}
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return systemProxy{}
	}
	return parseScutilProxy(string(out))
}

// useSystemProxy sets HTTPS_PROXY (and NO_PROXY) from macOS's settings,
// unless the environment already names a proxy, which wins.
func useSystemProxy() {
	if proxyFromEnv() != "" {
		return
	}
	p := readSystemProxy()
	if p.https == "" {
		return
	}
	_ = os.Setenv("HTTPS_PROXY", "http://"+p.https)
	if len(p.noProxy) > 0 && os.Getenv("NO_PROXY") == "" && os.Getenv("no_proxy") == "" {
		_ = os.Setenv("NO_PROXY", strings.Join(p.noProxy, ","))
	}
}

// proxyFromEnv is the proxy the environment names, if any.
func proxyFromEnv() string {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
