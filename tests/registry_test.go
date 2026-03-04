package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestRegistryUnmarshal(t *testing.T) {
	data := `{
		"version": 1,
		"tools": [
			{
				"name": "testtool",
				"description": "A test tool",
				"category": "Testing",
				"method": "apt",
				"package": "testtool",
				"binary": "testtool"
			}
		]
	}`

	var reg core.Registry
	if err := json.Unmarshal([]byte(data), &reg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reg.Version != 1 {
		t.Errorf("version = %d, want 1", reg.Version)
	}
	if len(reg.Tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(reg.Tools))
	}
	if reg.Tools[0].Name != "testtool" {
		t.Errorf("name = %q, want %q", reg.Tools[0].Name, "testtool")
	}
}

func TestRegistryValidation_BadName(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "../evil", Description: "bad", Category: "bad", Method: "apt", Package: "x", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for bad tool name")
	}
}

func TestRegistryValidation_UnknownMethod(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "snap", Package: "x", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestRegistryValidation_MissingPackage(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "apt", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for missing package")
	}
}

func TestRegistryValidation_MissingGitRepo(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "git_clone", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for missing git_repo")
	}
}

func TestRegistryValidation_MissingAppRepo(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "appimage", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for missing app_repo")
	}
}

func TestRegistryValidation_BadVersion(t *testing.T) {
	reg := &core.Registry{
		Version: 99,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "apt", Package: "x", Binary: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for bad version")
	}
}

func TestRegistryValidation_DuplicateName(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test1", Category: "test", Method: "apt", Package: "x", Binary: "x"},
			{Name: "test", Description: "test2", Category: "test", Method: "apt", Package: "y", Binary: "y"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for duplicate name")
	}
}

func TestRegistryValidation_MissingBinary(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "test", Description: "test", Category: "test", Method: "apt", Package: "x"},
		},
	}
	if err := core.ValidateRegistry(reg); err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestRegistryValidation_Valid(t *testing.T) {
	reg := &core.Registry{
		Version: 1,
		Tools: []core.RegistryTool{
			{Name: "tool-a", Description: "A", Category: "Cat", Method: "apt", Package: "a", Binary: "a"},
			{Name: "tool-b", Description: "B", Category: "Cat", Method: "go", Package: "github.com/x/y@latest", Binary: "y"},
			{Name: "tool-c", Description: "C", Category: "Cat", Method: "git_clone", Binary: "c", GitRepo: "https://github.com/x/y.git"},
			{Name: "tool-d", Description: "D", Category: "Cat", Method: "appimage", Binary: "d", AppRepo: "x/y"},
			{Name: "tool-e", Description: "E", Category: "Cat", Method: "pipx", Package: "e", Binary: "e"},
			{Name: "tool-f", Description: "F", Category: "Cat", Method: "cargo", Package: "f", Binary: "f"},
		},
	}
	if err := core.ValidateRegistry(reg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadCachedRegistry_FromFile(t *testing.T) {
	// Create a temp directory to simulate the cache
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "toolkit-registry.json")

	regData := `{
		"version": 1,
		"tools": [
			{"name": "test", "description": "test", "category": "test", "method": "apt", "package": "test", "binary": "test"}
		]
	}`
	if err := os.WriteFile(cachePath, []byte(regData), 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	// Read it back directly (testing the JSON parse, not the path logic)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var reg core.Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(reg.Tools) != 1 || reg.Tools[0].Name != "test" {
		t.Errorf("unexpected registry content: %+v", reg)
	}
}

func TestLoadCachedRegistry_MissingFile(t *testing.T) {
	// Temporarily set HOME to a dir without a cache
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := core.LoadCachedRegistry()
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestFetchRegistry_LocalFile(t *testing.T) {
	// Use temp HOME so FetchRegistry cache writes don't pollute real home
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	regFile := filepath.Join(tmpDir, "registry.json")

	regData := `{
		"version": 1,
		"tools": [
			{"name": "local-test", "description": "test", "category": "test", "method": "apt", "package": "test", "binary": "test"}
		]
	}`
	if err := os.WriteFile(regFile, []byte(regData), 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	reg, err := core.FetchRegistry(regFile)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(reg.Tools) != 1 || reg.Tools[0].Name != "local-test" {
		t.Errorf("unexpected registry: %+v", reg)
	}
}

func TestFetchRegistry_FileURL(t *testing.T) {
	// Use temp HOME so FetchRegistry cache writes don't pollute real home
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	regFile := filepath.Join(tmpDir, "registry.json")

	regData := `{
		"version": 1,
		"tools": [
			{"name": "fileurl-test", "description": "test", "category": "test", "method": "apt", "package": "test", "binary": "test"}
		]
	}`
	if err := os.WriteFile(regFile, []byte(regData), 0644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	reg, err := core.FetchRegistry("file://" + regFile)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(reg.Tools) != 1 || reg.Tools[0].Name != "fileurl-test" {
		t.Errorf("unexpected registry: %+v", reg)
	}
}

func TestValidToolName(t *testing.T) {
	valid := []string{"nmap", "ffuf", "tool-a", "tool_b", "Go2", "a1-b2_c3"}
	for _, name := range valid {
		if !core.ValidToolName.MatchString(name) {
			t.Errorf("%q should be valid", name)
		}
	}

	invalid := []string{"", "-nmap", "_nmap", "../evil", "has space", "has;semi"}
	for _, name := range invalid {
		if core.ValidToolName.MatchString(name) {
			t.Errorf("%q should be invalid", name)
		}
	}
}
