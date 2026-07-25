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
// Also applied to Binary, which becomes a filename.
var ValidToolName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// The registry is remote by default and fully overridable (--registry, or
// toolkit_registry_url), so every field below is untrusted input that reaches
// either an exec argument or a filesystem path. The anchors matter:
//
//   - a leading alphanumeric stops a value being parsed as a command-line
//     option (`-o=APT::...`, `--upload-pack=...`)
//   - constraining the git scheme to https blocks git's remote helpers
//     (`ext::sh -c ...`), which execute commands
//   - rejecting ".." keeps repo slugs from redirecting the GitHub API URL they
//     are interpolated into, and keeps paths inside their intended directory
var (
	// validRepoSlug matches a GitHub "owner/repo" pair.
	validRepoSlug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)

	// validGitURL matches an https:// clone URL.
	validGitURL = regexp.MustCompile(`^https://[A-Za-z0-9][A-Za-z0-9.-]*/[A-Za-z0-9._/-]+$`)

	// validPackage matches a package spec for apt/go/cargo/pipx. The charset
	// covers real specs such as "github.com/OJ/gobuster/v3@latest" and
	// "git+https://github.com/Pennyw0rth/NetExec".
	validPackage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@+:-]*$`)

	// validAssetPattern matches a GitHub release asset substring.
	validAssetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._*-]*$`)
)

// checkField validates one untrusted registry field.
func checkField(tool, field, value string, re *regexp.Regexp) error {
	if !re.MatchString(value) {
		return fmt.Errorf("tool %q: invalid %s %q", tool, field, value)
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("tool %q: %s must not contain \"..\": %q", tool, field, value)
	}
	return nil
}

// validMethods lists the allowed install method strings.
var validMethods = map[string]bool{
	"apt":            true,
	"go":             true,
	"pipx":           true,
	"cargo":          true,
	"git_clone":      true,
	"appimage":       true,
	"deb":            true,
	"release_binary": true,
	"rustup":         true,
}

// validDistros lists the allowed distro filter strings.
var validDistros = map[string]bool{
	"debian": true,
	"arch":   true,
	"fedora": true,
}

// RegistryTool describes a single toolkit tool's metadata.
type RegistryTool struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Method       string   `json:"method"`
	Package      string   `json:"package,omitempty"`
	Binary       string   `json:"binary"`
	AppRepo      string   `json:"app_repo,omitempty"`
	GitRepo      string   `json:"git_repo,omitempty"`
	DebRepo      string   `json:"deb_repo,omitempty"`
	ReleaseRepo  string   `json:"release_repo,omitempty"`
	AssetPattern string   `json:"asset_pattern,omitempty"`
	Distros      []string `json:"distros,omitempty"`
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

	// Validate on read, not just on fetch. The cache is a plain file on disk
	// and LoadOrFetchRegistry prefers it, so skipping this would let an edited
	// or truncated cache bypass the trust boundary entirely.
	if err := ValidateRegistry(&reg); err != nil {
		return nil, fmt.Errorf("invalid cached registry: %w", err)
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
		// Binary is joined into ~/.local/bin and ~/.local/share/toolkit, is
		// chmod 0755'd after download, and is os.RemoveAll'd on uninstall — so
		// it must be a bare filename, not a path.
		if err := checkField(t.Name, "binary", t.Binary, ValidToolName); err != nil {
			return err
		}

		// requiredField pairs each method with the field it consumes and the
		// pattern that field must satisfy.
		var (
			field string
			value string
			re    *regexp.Regexp
		)
		switch t.Method {
		case "apt", "go", "pipx", "cargo":
			field, value, re = "package", t.Package, validPackage
		case "git_clone":
			field, value, re = "git_repo", t.GitRepo, validGitURL
		case "appimage":
			field, value, re = "app_repo", t.AppRepo, validRepoSlug
		case "deb":
			field, value, re = "deb_repo", t.DebRepo, validRepoSlug
		case "release_binary":
			field, value, re = "release_repo", t.ReleaseRepo, validRepoSlug
		}
		if field != "" {
			if value == "" {
				return fmt.Errorf("tool %q: %s is required for %s method", t.Name, field, t.Method)
			}
			if err := checkField(t.Name, field, value, re); err != nil {
				return err
			}
		}

		if t.AssetPattern != "" {
			if err := checkField(t.Name, "asset_pattern", t.AssetPattern, validAssetPattern); err != nil {
				return err
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
