package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const brewPlugins = "/opt/homebrew/lib/docker/cli-plugins"

func readDockerConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("written config is not JSON: %v\n%s", err, data)
	}
	return cfg
}

func TestAddDockerPluginDir_CreatesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".docker", "config.json")
	if err := addDockerPluginDir(path, brewPlugins); err != nil {
		t.Fatal(err)
	}
	dirs, _ := readDockerConfig(t, path)["cliPluginsExtraDirs"].([]any)
	if len(dirs) != 1 || dirs[0] != brewPlugins {
		t.Errorf("cliPluginsExtraDirs = %v, want [%s]", dirs, brewPlugins)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %v (err %v), want 0600: it can hold registry credentials", fi.Mode().Perm(), err)
	}
}

// The docker config carries registry auths and contexts. Registering the plugin
// dir must leave all of it in place, including numbers json would otherwise
// round through float64.
func TestAddDockerPluginDir_KeepsExistingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	orig := `{"auths":{"ghcr.io":{"auth":"c2VjcmV0"}},"currentContext":"colima","cliPluginsExtraDirs":["/other"],"n":12345678901234567890}`
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addDockerPluginDir(path, brewPlugins); err != nil {
		t.Fatal(err)
	}
	cfg := readDockerConfig(t, path)
	auth := cfg["auths"].(map[string]any)["ghcr.io"].(map[string]any)["auth"]
	if auth != "c2VjcmV0" || cfg["currentContext"] != "colima" {
		t.Errorf("existing settings lost: %v", cfg)
	}
	dirs := cfg["cliPluginsExtraDirs"].([]any)
	if len(dirs) != 2 || dirs[0] != "/other" || dirs[1] != brewPlugins {
		t.Errorf("cliPluginsExtraDirs = %v, want [/other %s]", dirs, brewPlugins)
	}
	raw, _ := os.ReadFile(path)
	if !json.Valid(raw) || !strings.Contains(string(raw), "12345678901234567890") {
		t.Errorf("large number was not preserved exactly:\n%s", raw)
	}
}

func TestAddDockerPluginDir_AlreadyListedIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	orig := `{"cliPluginsExtraDirs": ["` + brewPlugins + `"]}`
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addDockerPluginDir(path, brewPlugins); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != orig {
		t.Errorf("file rewritten although the dir was already listed:\n%s", got)
	}
}

func TestAddDockerPluginDir_RefusesWhatItCannotParse(t *testing.T) {
	for name, orig := range map[string]string{
		"malformed":      `{"auths": {`,
		"dirs not array": `{"cliPluginsExtraDirs": "/opt/homebrew/lib/docker/cli-plugins"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := addDockerPluginDir(path, brewPlugins); err == nil {
				t.Error("want an error")
			}
			if got, _ := os.ReadFile(path); string(got) != orig {
				t.Errorf("file changed on error:\n%s", got)
			}
		})
	}
}
