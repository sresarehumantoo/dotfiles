package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultRegistryURL is the raw GitHub URL for the toolkit registry.
const DefaultRegistryURL = "https://raw.githubusercontent.com/sresarehumantoo/dotfiles-toolkit/main/registry.json"

// ValidToolName matches safe tool names (alphanumeric, hyphens, underscores).
var ValidToolName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// validMethods lists the allowed install method strings.
var validMethods = map[string]bool{
	"apt":       true,
	"go":        true,
	"pipx":      true,
	"cargo":     true,
	"git_clone": true,
	"appimage":  true,
	"deb":       true,
}

// validDistros lists the allowed distro filter strings.
var validDistros = map[string]bool{
	"debian": true,
	"arch":   true,
	"fedora": true,
}

// RegistryTool describes a single toolkit tool's metadata.
type RegistryTool struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Method      string   `json:"method"`
	Package     string   `json:"package,omitempty"`
	Binary      string   `json:"binary"`
	AppRepo     string   `json:"app_repo,omitempty"`
	GitRepo     string   `json:"git_repo,omitempty"`
	DebRepo     string   `json:"deb_repo,omitempty"`
	Distros     []string `json:"distros,omitempty"`
}

// Registry is the top-level structure of the toolkit registry JSON.
type Registry struct {
	Version int            `json:"version"`
	Tools   []RegistryTool `json:"tools"`
}

// RegistryCachePath returns the path to the cached toolkit registry.
func RegistryCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dfinstall", "toolkit-registry.json")
}

// FetchRegistry downloads the registry from a URL and writes it to the cache.
func FetchRegistry(url string) (*Registry, error) {
	Debug("fetching registry from %s", url)

	var data []byte
	var err error

	if strings.HasPrefix(url, "file://") {
		// Local file path
		path := strings.TrimPrefix(url, "file://")
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read local registry %s: %w", path, err)
		}
	} else if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Treat as a plain file path
		data, err = os.ReadFile(url)
		if err != nil {
			return nil, fmt.Errorf("read local registry %s: %w", url, err)
		}
	} else {
		// HTTP(S) URL — fetch with curl
		data, err = exec.Command("curl", "-fsSL", url).Output()
		if err != nil {
			return nil, fmt.Errorf("fetch registry from %s: %w", url, err)
		}
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry JSON: %w", err)
	}

	if err := ValidateRegistry(&reg); err != nil {
		return nil, fmt.Errorf("invalid registry: %w", err)
	}

	// Write to cache
	cachePath := RegistryCachePath()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		Warn("failed to create registry cache dir: %v", err)
	} else if err := os.WriteFile(cachePath, data, 0644); err != nil {
		Warn("failed to write registry cache: %v", err)
	} else {
		Debug("registry cached to %s", cachePath)
	}

	return &reg, nil
}

// CleanRegistryCache removes the cached registry file from disk.
func CleanRegistryCache() {
	cachePath := RegistryCachePath()
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		Debug("clean registry cache: %v", err)
	} else if err == nil {
		Debug("registry cache removed: %s", cachePath)
	}
}

// LoadCachedRegistry reads the registry from the local cache file.
func LoadCachedRegistry() (*Registry, error) {
	cachePath := RegistryCachePath()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("read registry cache: %w", err)
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse cached registry: %w", err)
	}

	return &reg, nil
}

// LoadOrFetchRegistry loads the registry from cache or fetches it remotely.
// If forceRefresh is true, always fetches from the remote URL.
func LoadOrFetchRegistry(forceRefresh bool) (*Registry, error) {
	url := Cfg.ToolkitRegistryURL
	if url == "" {
		url = DefaultRegistryURL
	}

	if forceRefresh {
		return FetchRegistry(url)
	}

	// Try cache first
	reg, err := LoadCachedRegistry()
	if err == nil {
		return reg, nil
	}

	// No cache — fetch
	return FetchRegistry(url)
}

// ValidateRegistry checks the registry for correctness.
func ValidateRegistry(r *Registry) error {
	if r.Version != 1 {
		return fmt.Errorf("unsupported registry version %d (expected 1)", r.Version)
	}

	if len(r.Tools) == 0 {
		return fmt.Errorf("registry has no tools")
	}

	seen := make(map[string]bool)
	for i, t := range r.Tools {
		if !ValidToolName.MatchString(t.Name) {
			return fmt.Errorf("tool %d: invalid name %q", i, t.Name)
		}
		if seen[t.Name] {
			return fmt.Errorf("tool %d: duplicate name %q", i, t.Name)
		}
		seen[t.Name] = true

		if t.Category == "" {
			return fmt.Errorf("tool %q: category is required", t.Name)
		}

		if !validMethods[t.Method] {
			return fmt.Errorf("tool %q: unknown method %q", t.Name, t.Method)
		}

		if t.Binary == "" {
			return fmt.Errorf("tool %q: binary is required", t.Name)
		}

		switch t.Method {
		case "apt":
			if t.Package == "" {
				return fmt.Errorf("tool %q: package is required for apt method", t.Name)
			}
		case "go":
			if t.Package == "" {
				return fmt.Errorf("tool %q: package is required for go method", t.Name)
			}
		case "pipx":
			if t.Package == "" {
				return fmt.Errorf("tool %q: package is required for pipx method", t.Name)
			}
		case "cargo":
			if t.Package == "" {
				return fmt.Errorf("tool %q: package is required for cargo method", t.Name)
			}
		case "git_clone":
			if t.GitRepo == "" {
				return fmt.Errorf("tool %q: git_repo is required for git_clone method", t.Name)
			}
		case "appimage":
			if t.AppRepo == "" {
				return fmt.Errorf("tool %q: app_repo is required for appimage method", t.Name)
			}
		case "deb":
			if t.DebRepo == "" {
				return fmt.Errorf("tool %q: deb_repo is required for deb method", t.Name)
			}
		}

		for _, d := range t.Distros {
			if !validDistros[d] {
				return fmt.Errorf("tool %q: unknown distro filter %q", t.Name, d)
			}
		}
	}

	return nil
}

// ToolMatchesDistro returns true if the tool is available on the current distro.
// Tools with no distros filter match all distros.
func ToolMatchesDistro(t RegistryTool) bool {
	if len(t.Distros) == 0 {
		return true
	}
	d := GetDistro()
	for _, filter := range t.Distros {
		switch filter {
		case "debian":
			if d == DistroDebian {
				return true
			}
		case "arch":
			if d == DistroArch || d == DistroSteamOS {
				return true
			}
		case "fedora":
			if d == DistroFedora {
				return true
			}
		}
	}
	return false
}
