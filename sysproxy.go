package main

import (
	"bufio"
	"net"
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

// parseScutilProxy reads `scutil --proxy` output: the top-level settings
// only, not the per-interface copies under __SCOPED__, which would
// otherwise overwrite them.
func parseScutilProxy(out string) systemProxy {
	var p systemProxy
	vals := map[string]string{}
	depth := 0         // 1 is the top-level dictionary
	exceptionsAt := -1 // the depth of the top-level ExceptionsList, while in it
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, kv := strings.Cut(line, " : ")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch {
		case strings.HasSuffix(line, "{"):
			depth++
			if depth == 2 && kv && k == "ExceptionsList" {
				exceptionsAt = depth
			}
		case line == "}":
			if depth == exceptionsAt {
				exceptionsAt = -1
			}
			depth--
		case depth == exceptionsAt && kv:
			if host := noProxyEntry(v); host != "" {
				p.noProxy = append(p.noProxy, host)
			}
		case depth == 1 && kv:
			vals[k] = v
		}
	}
	if vals["HTTPSEnable"] == "1" && vals["HTTPSProxy"] != "" {
		p.https = vals["HTTPSProxy"]
		if port := vals["HTTPSPort"]; port != "" {
			p.https = net.JoinHostPort(p.https, port)
		}
	}
	if vals["ProxyAutoConfigEnable"] == "1" {
		p.pac = vals["ProxyAutoConfigURLString"]
	}
	return p
}

// noProxyEntry turns a macOS exception ("*.example.com", "10.0.0.0/8",
// "fd00::/8") into a NO_PROXY entry, or "" for the abbreviated network
// forms ("169.254/16") NO_PROXY can't express.
func noProxyEntry(v string) string {
	if strings.Contains(v, "/") {
		if _, _, err := net.ParseCIDR(v); err != nil {
			return ""
		}
		return v
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
	if proxyEnvSet() {
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

// proxyEnvSet reports whether the environment says anything about a proxy,
// HTTPS_PROXY= (empty: none) included, which then wins over macOS's.
func proxyEnvSet() bool {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy"} {
		if _, ok := os.LookupEnv(k); ok {
			return true
		}
	}
	return false
}
