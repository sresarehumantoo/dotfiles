package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/internal/core"
	"github.com/sresarehumantoo/dotfiles/internal/modules"
	"github.com/tailscale/hujson"
)

const opencodeBase = `{
  // comments and trailing commas are allowed in the tracked base
  "enabled_providers": ["anthropic", "amazon-bedrock"],
  "disabled_providers": ["opencode"],
  "share": "disabled",
  "instructions": ["~/.claude/CLAUDE.local.md"],
  "permission": {"edit": {"*": "ask", "/tmp/**": "allow"}, "bash": {"*": "ask"}},
}`

// opencodeSandbox points HOME, XDG_CONFIG_HOME and DOTFILES at temp dirs and
// puts a stand-in opencode first on PATH, so Install takes the "installed
// elsewhere" branch and never touches the network.
func opencodeSandbox(t *testing.T) (home, cfg string) {
	t.Helper()
	home = t.TempDir()
	dots := t.TempDir()
	cfg = filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("DOTFILES", dots)
	core.ResetDotfilesDir()
	t.Cleanup(core.ResetDotfilesDir)

	for _, d := range []string{"opencode", "claude"} {
		if err := os.MkdirAll(filepath.Join(dots, "config", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dots, "config", "opencode", "opencode.jsonc"), []byte(opencodeBase), 0o644)
	os.WriteFile(filepath.Join(dots, "config", "claude", "CLAUDE.md"), []byte("# rules\n"), 0o644)

	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\necho 1.0.0\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	saved := core.Cfg.OpencodeEnabled
	core.Cfg.OpencodeEnabled = true
	t.Cleanup(func() { core.Cfg.OpencodeEnabled = saved })
	return home, cfg
}

func TestOpencodeInstallMergesLinksAndPins(t *testing.T) {
	_, cfg := opencodeSandbox(t)
	live := filepath.Join(cfg, "opencode", "opencode.json")
	os.MkdirAll(filepath.Dir(live), 0o755)
	os.WriteFile(live, []byte(`{"model": "amazon-bedrock/x", "share": "auto"}`), 0o600)

	m := modules.OpencodeModule{}
	if err := m.Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, _ := os.ReadFile(live)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("live opencode.json is not plain JSON: %v\n%s", err, data)
	}
	if got["model"] != "amazon-bedrock/x" {
		t.Errorf("unmanaged key lost: %v", got["model"])
	}
	if got["share"] != "disabled" {
		t.Errorf("share = %v, want the managed value", got["share"])
	}

	agents := filepath.Join(cfg, "opencode", "AGENTS.md")
	if dst, err := os.Readlink(agents); err != nil || filepath.Base(dst) != "CLAUDE.md" {
		t.Errorf("AGENTS.md link = %q, %v", dst, err)
	}

	pin := opencodePin(t, filepath.Join(cfg, "dfinstall", "opencode.zsh"))
	if _, ok := pin["instructions"]; ok {
		t.Error("instructions must not be pinned: arrays in the pin replace a project's own")
	}
	if !reflect.DeepEqual(pin["enabled_providers"], []any{"anthropic", "amazon-bedrock"}) || pin["share"] != "disabled" {
		t.Errorf("pin = %v", pin)
	}

	if s := m.Status(); s.Linked != 1 || s.Extra != "" {
		t.Errorf("after Install: linked=%d extra=%q", s.Linked, s.Extra)
	}
	if d := m.ContentDrift(); d != nil {
		t.Errorf("after Install: drift %v", d)
	}
}

// opencodePin returns the JSON exported as OPENCODE_CONFIG_CONTENT, and checks
// the env switches next to it.
func opencodePin(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"export OPENCODE_PURE=1", "export OPENCODE_DISABLE_FREE_MODELS=1", "alias opencode-untrusted="} {
		if !strings.Contains(string(data), want) {
			t.Errorf("%s lacks %q", path, want)
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		v, ok := strings.CutPrefix(line, "export OPENCODE_CONFIG_CONTENT=")
		if !ok {
			continue
		}
		v = strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(v, "'"), "'"), `'\''`, "'")
		var pin map[string]any
		if err := json.Unmarshal([]byte(v), &pin); err != nil {
			t.Fatalf("pin is not JSON: %v\n%s", err, v)
		}
		return pin
	}
	t.Fatalf("%s exports no OPENCODE_CONFIG_CONTENT", path)
	return nil
}

// opencode loads opencode.jsonc after opencode.json, so merging around one
// would leave every managed key overridden while status reported success.
func TestOpencodeInstallRefusesOverridingFile(t *testing.T) {
	_, cfg := opencodeSandbox(t)
	jsonc := filepath.Join(cfg, "opencode", "opencode.jsonc")
	os.MkdirAll(filepath.Dir(jsonc), 0o755)
	os.WriteFile(jsonc, []byte(`{"share": "auto"}`), 0o600)

	m := modules.OpencodeModule{}
	if err := m.Install(context.Background()); err == nil {
		t.Fatal("Install succeeded beside an opencode.jsonc")
	}
	if _, err := os.Stat(filepath.Join(cfg, "opencode", "opencode.json")); err == nil {
		t.Error("Install wrote opencode.json despite refusing")
	}
	if s := m.Status(); !strings.Contains(s.Extra, "opencode.jsonc overrides opencode.json") {
		t.Errorf("Status().Extra = %q", s.Extra)
	}
	if d := m.ContentDrift(); len(d) != 1 || d[0].Dst != jsonc {
		t.Errorf("ContentDrift = %+v", d)
	}
}

// opencode writes this file itself on a first run with no config; it must not
// block the install.
func TestOpencodeInstallIgnoresSchemaOnlyJsonc(t *testing.T) {
	_, cfg := opencodeSandbox(t)
	jsonc := filepath.Join(cfg, "opencode", "opencode.jsonc")
	os.MkdirAll(filepath.Dir(jsonc), 0o755)
	os.WriteFile(jsonc, []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}"), 0o644)

	m := modules.OpencodeModule{}
	if err := m.Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if d := m.ContentDrift(); d != nil {
		t.Errorf("ContentDrift = %+v", d)
	}
}

func TestOpencodeDryRunWritesNothing(t *testing.T) {
	_, cfg := opencodeSandbox(t)
	core.DryRun = true
	t.Cleanup(func() { core.DryRun = false })

	if err := (modules.OpencodeModule{}).Install(context.Background()); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, p := range []string{
		filepath.Join(cfg, "opencode"),
		filepath.Join(cfg, "dfinstall", "opencode.zsh"),
	} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("dry run created %s", p)
		}
	}
}

func TestOpencodeStatusDisabledUntilOptIn(t *testing.T) {
	opencodeSandbox(t)
	core.Cfg.OpencodeEnabled = false
	if s := (modules.OpencodeModule{}).Status(); s.Extra != "disabled" || s.Linked+s.Missing != 0 {
		t.Errorf("Status = %+v, want disabled", s)
	}
}

func TestOpencodeUninstallKeepsSettings(t *testing.T) {
	home, cfg := opencodeSandbox(t)
	m := modules.OpencodeModule{}
	if err := m.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(home, ".local", "bin", "opencode")
	npmDir := filepath.Join(home, ".local", "share", "dfinstall", "opencode", "node_modules")
	os.MkdirAll(npmDir, 0o755)
	os.MkdirAll(filepath.Dir(local), 0o755)
	os.Symlink(filepath.Join(npmDir, "x"), local)

	if err := m.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	for _, p := range []string{
		filepath.Join(cfg, "opencode", "AGENTS.md"),
		filepath.Join(cfg, "dfinstall", "opencode.zsh"),
		local,
		filepath.Dir(npmDir),
	} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("Uninstall left %s", p)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg, "opencode", "opencode.json")); err != nil {
		t.Errorf("Uninstall removed the merged settings: %v", err)
	}
}

// The shipped config is the security boundary for this module; loosening it
// should be a deliberate edit to this test too.
func TestOpencodeShippedConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "config", "opencode", "opencode.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	std, err := hujson.Standardize(data)
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		EnabledProviders  []string                   `json:"enabled_providers"`
		DisabledProviders []string                   `json:"disabled_providers"`
		Share             string                     `json:"share"`
		Plugin            []string                   `json:"plugin"`
		Permission        map[string]json.RawMessage `json:"permission"`
	}
	if err := json.Unmarshal(std, &c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.EnabledProviders, []string{"anthropic", "amazon-bedrock"}) {
		t.Errorf("enabled_providers = %v", c.EnabledProviders)
	}
	if !reflect.DeepEqual(c.DisabledProviders, []string{"opencode"}) {
		t.Errorf("disabled_providers = %v, want Zen off", c.DisabledProviders)
	}
	if c.Share != "disabled" || c.Plugin == nil || len(c.Plugin) != 0 {
		t.Errorf("share = %q, plugin = %v", c.Share, c.Plugin)
	}
	for _, tool := range []string{"edit", "bash"} {
		var rules map[string]string
		if err := json.Unmarshal(c.Permission[tool], &rules); err != nil {
			t.Fatalf("permission.%s: %v", tool, err)
		}
		if rules["*"] != "ask" {
			t.Errorf("permission.%s[*] = %q, want ask (Claude Code's default)", tool, rules["*"])
		}
	}
}
