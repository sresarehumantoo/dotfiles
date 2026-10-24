package modules

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// Package mirrors (Nexus, Artifactory) as configured for each package manager
// dfinstall drives.
//
// Nothing here configures a mirror: a corporate machine already points npm,
// pip, Go and cargo at one, and each tool honors its own config. pipx, `go
// install` and `cargo install` therefore go through the mirror with no help
// from us. What dfinstall decides is where to get a tool that is published on
// more than one channel: when npm is mirrored, an npm package beats a GitHub
// release download that a corporate proxy may block.

const publicNpmRegistry = "https://registry.npmjs.org/"

// npmRegistry returns the registry npm resolves packages from, as npm itself
// reports it, so ~/.npmrc, a project .npmrc and NPM_CONFIG_REGISTRY all count.
// "" when npm is not installed or does not answer.
func npmRegistry(ctx context.Context) string {
	if _, err := exec.LookPath("npm"); err != nil {
		return ""
	}
	out, err := runProbe(ctx, "npm", "config", "get", "registry")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// npmMirrored reports whether registry is somewhere other than the public npm
// registry.
func npmMirrored(registry string) bool {
	if registry == "" {
		return false
	}
	return strings.TrimSuffix(registry, "/") != strings.TrimSuffix(publicNpmRegistry, "/")
}

// pipIndex returns pip's configured index-url, or "" for the default PyPI.
func pipIndex(ctx context.Context) string {
	if v := os.Getenv("PIP_INDEX_URL"); v != "" {
		return v
	}
	if _, err := exec.LookPath("pip3"); err != nil {
		return ""
	}
	out, err := runProbe(ctx, "pip3", "config", "list")
	if err != nil {
		return ""
	}
	// Lines look like: global.index-url='https://nexus.example/repository/pypi/simple'
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok && strings.HasSuffix(k, ".index-url") {
			return strings.Trim(strings.TrimSpace(v), `'"`)
		}
	}
	return ""
}

// goProxy returns GOPROXY when it is not the default.
func goProxy(ctx context.Context) string {
	if _, err := exec.LookPath("go"); err != nil {
		return ""
	}
	out, err := runProbe(ctx, "go", "env", "GOPROXY")
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if v == "https://proxy.golang.org,direct" {
		return ""
	}
	return v
}

// cargoMirror returns the registry crates-io is replaced with in cargo's
// config, or "".
func cargoMirror() string {
	home := os.Getenv("CARGO_HOME")
	if home == "" {
		home = core.HomeTarget(".cargo")
	}
	if home == "" {
		return ""
	}
	for _, name := range []string{"config.toml", "config"} {
		if m := cargoReplacement(filepath.Join(home, name)); m != "" {
			return m
		}
	}
	return ""
}

// cargoReplacement reads one cargo config file. A source-replacement block
// names the mirror in a `registry = "..."` line under [source.<name>];
// `replace-with` alone says one is in use.
func cargoReplacement(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	var replaced bool
	var registry string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "replace-with":
			replaced = true
		case "registry":
			registry = strings.Trim(strings.TrimSpace(v), `'"`)
		}
	}
	switch {
	case !replaced:
		return ""
	case registry == "":
		return "source replacement in " + path
	}
	return registry
}

// redactURL drops credentials from a URL before it is printed: an .npmrc or
// pip.conf may embed them as user:pass@host.
func redactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	u.User = nil
	return u.String()
}

// mirrorsDoctorResult reports which package managers go through a mirror. It
// never fails: a mirror is configuration, not a fault.
func mirrorsDoctorResult() DoctorResult {
	// No context to inherit: doctor runs as a synchronous report.
	ctx := context.Background()
	rows := []struct{ name, url string }{
		{"npm", ""},
		{"pip", pipIndex(ctx)},
		{"go", goProxy(ctx)},
		{"cargo", cargoMirror()},
	}
	if reg := npmRegistry(ctx); npmMirrored(reg) {
		rows[0].url = reg
	}

	// Passing rows print only their detail line, so the URLs go there.
	var mirrored []string
	for _, r := range rows {
		if r.url != "" {
			mirrored = append(mirrored, fmt.Sprintf("%s %s", r.name, redactURL(r.url)))
		}
	}
	if len(mirrored) == 0 {
		return DoctorResult{Name: "package mirrors", OK: true, Detail: "none, public registries"}
	}
	return DoctorResult{Name: "package mirrors", OK: true, Detail: strings.Join(mirrored, "; ")}
}
