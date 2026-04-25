package modules

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// DiscoveredFile represents a custom shell file found in $HOME.
type DiscoveredFile struct {
	// RelPath is the path relative to $HOME (e.g. ".companyrc").
	RelPath string
}

// shellGlobs are the glob patterns to search for in $HOME.
var shellGlobs = []string{
	".*rc",
	".*aliases*",
	".*functions*",
	".*_profile",
	".*_env",
	".localrc",
	".shellrc",
	".env.local",
}

// managedDestinations are the shell link destinations that dfinstall manages.
// Files matching these are excluded from the scan.
var managedDestinations map[string]bool

func init() {
	managedDestinations = make(map[string]bool)
	for _, l := range shellLinks {
		managedDestinations[l.dst] = true
	}
}

// nonShellFiles are dotfiles that match our globs but aren't shell configs.
var nonShellFiles = map[string]bool{
	".vimrc":        true,
	".npmrc":        true,
	".netrc":        true,
	".wgetrc":       true,
	".curlrc":       true,
	".inputrc":      true,
	".screenrc":     true,
	".nanorc":       true,
	".editrc":       true,
	".tigrc":        true,
	".procmailrc":   true,
	".perlcriticrc": true,
	".pylintrc":     true,
	".flake8rc":     true,
	".claborc":      true,
	".dockerrc":     true,
	".gemrc":        true,
	".irbrc":        true,
	".pryrc":        true,
	".sqliterc":     true,
	".psqlrc":       true,
	".myclirc":      true,
	".pgclirc":      true,
	".lftprc":       true,
	".muttrc":       true,
	".mbsyncrc":     true,
	".dmrc":         true, // display manager session record (INI)
	".gtkrc":        true, // GTK1 config (key=value)
	".asoundrc":     true, // ALSA config
	".face":         true, // user avatar (binary)
}

// iniSectionLine matches an INI section header like [Section] or [Foo.Bar].
// Excludes shell test syntax like `[ -f foo ]` (requires no space after `[`).
var iniSectionLine = regexp.MustCompile(`^\[[a-zA-Z0-9][a-zA-Z0-9 _.-]*\]\s*$`)

// looksSourceable does a best-effort content sniff to decide whether a file
// could plausibly be sourced by a POSIX shell. Returns false for binary files,
// INI files (e.g. .dmrc), and XML files. Default-accepts on read errors so we
// fall back to the user's judgement in the menu.
func looksSourceable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	buf = buf[:n]

	if bytes.IndexByte(buf, 0) != -1 {
		return false
	}

	for _, line := range strings.Split(string(buf), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if iniSectionLine.MatchString(line) {
			return false
		}
		if strings.HasPrefix(line, "<?xml") || strings.HasPrefix(line, "<!DOCTYPE") {
			return false
		}
		break
	}
	return true
}

// validPreservePath matches safe relative paths (dotfiles in $HOME, no slashes, no injection).
var validPreservePath = regexp.MustCompile(`^\.[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// escKeyMap returns the default huh keymap with ESC/q added to the Quit binding.
func escKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c", "q"))
	return km
}

// ScanCustomShellFiles globs $HOME for shell-like dotfiles that aren't managed by dfinstall.
func ScanCustomShellFiles() []DiscoveredFile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var results []DiscoveredFile

	for _, pattern := range shellGlobs {
		matches, err := filepath.Glob(filepath.Join(home, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			rel, err := filepath.Rel(home, match)
			if err != nil {
				continue
			}

			// Skip duplicates
			if seen[rel] {
				continue
			}
			seen[rel] = true

			// Skip managed destinations
			if managedDestinations[rel] {
				continue
			}

			// Skip known non-shell files
			if nonShellFiles[rel] {
				continue
			}

			fi, err := os.Lstat(match)
			if err != nil {
				continue
			}

			// Skip directories
			if fi.IsDir() {
				continue
			}

			// Skip symlinks (already managed by something)
			if fi.Mode()&os.ModeSymlink != 0 {
				continue
			}

			// Skip files > 1MB (not a shell config)
			if fi.Size() > 1<<20 {
				continue
			}

			// Skip files whose contents don't look shell-sourceable
			// (binary, INI like .dmrc, XML, etc.)
			if !looksSourceable(match) {
				continue
			}

			results = append(results, DiscoveredFile{RelPath: rel})
		}
	}

	return results
}

// SanitizePreservedFiles drops entries that no longer exist in $HOME or
// fail the shell-sourceable content sniff. Returns the cleaned slice and the
// list of dropped entries (for logging). Used to auto-heal config from
// before the sourceable sniffer existed (e.g. .dmrc previously ended up in
// preserved_files and now causes shell errors at startup).
func SanitizePreservedFiles(paths []string) (cleaned, dropped []string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return paths, nil
	}
	for _, p := range paths {
		full := filepath.Join(home, p)
		fi, err := os.Stat(full)
		if err != nil {
			dropped = append(dropped, p)
			continue
		}
		if fi.IsDir() {
			dropped = append(dropped, p)
			continue
		}
		if !looksSourceable(full) {
			dropped = append(dropped, p)
			continue
		}
		cleaned = append(cleaned, p)
	}
	return cleaned, dropped
}

// FilterNewFiles returns only files not already in PreservedFiles or DismissedFiles.
func FilterNewFiles(discovered []DiscoveredFile) []DiscoveredFile {
	known := make(map[string]bool)
	for _, p := range core.Cfg.PreservedFiles {
		known[p] = true
	}
	for _, p := range core.Cfg.DismissedFiles {
		known[p] = true
	}

	var result []DiscoveredFile
	for _, d := range discovered {
		if !known[d.RelPath] {
			result = append(result, d)
		}
	}
	return result
}

// RunPreserveMenu shows an interactive multi-select for discovered shell files.
// Returns the preserved and dismissed file paths.
func RunPreserveMenu(files []DiscoveredFile) (preserved, dismissed []string, err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		core.Warn("stdin is not a terminal — skipping custom file preservation menu")
		return nil, nil, nil
	}

	var options []huh.Option[string]
	for _, f := range files {
		options = append(options, huh.NewOption(f.RelPath, f.RelPath))
	}

	// Pre-select all files by default
	selected := make([]string, len(files))
	for i, f := range files {
		selected[i] = f.RelPath
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Custom Shell Files Found").
				Description("These files were found in ~/ and may be sourced by your current shell.\nSelect files to keep sourcing after dfinstall replaces your zshrc.\nSpace to toggle, Enter to confirm, Esc to skip all.").
				Options(options...).
				Value(&selected),
		),
	).WithKeyMap(escKeyMap())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			core.PrintHint("Selection cancelled — no custom files will be sourced")
			// Treat all as dismissed so we don't re-prompt
			for _, f := range files {
				dismissed = append(dismissed, f.RelPath)
			}
			return nil, dismissed, nil
		}
		return nil, nil, fmt.Errorf("preserve menu: %w", err)
	}

	selectedSet := make(map[string]bool, len(selected))
	for _, s := range selected {
		selectedSet[s] = true
	}

	for _, f := range files {
		if selectedSet[f.RelPath] {
			preserved = append(preserved, f.RelPath)
		} else {
			dismissed = append(dismissed, f.RelPath)
		}
	}

	if len(preserved) > 0 {
		core.Status("Preserving: %s", strings.Join(preserved, ", "))
	}
	if len(dismissed) > 0 {
		core.Status("Dismissed: %s", strings.Join(dismissed, ", "))
	}

	return preserved, dismissed, nil
}

// WriteCustomSourcesFile writes the generated custom-sources.zsh sourced by zshrc.
func WriteCustomSourcesFile(paths []string) error {
	dir := filepath.Join(core.XDGConfigHome(), "dfinstall")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dfinstall config dir: %w", err)
	}

	path := filepath.Join(dir, "custom-sources.zsh")

	// Validate paths before writing to a shell-sourced file
	for _, p := range paths {
		if !validPreservePath.MatchString(p) {
			return fmt.Errorf("invalid preserved path %q — must be a dotfile name (no slashes or special chars)", p)
		}
	}

	var sb strings.Builder
	sb.WriteString("# Generated by dfinstall — do not edit manually.\n")
	sb.WriteString("# Re-run: dfinstall install shell\n\n")

	for _, p := range paths {
		fmt.Fprintf(&sb, "[[ -f ~/%s ]] && source ~/%s\n", p, p)
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write custom-sources.zsh: %w", err)
	}

	core.Ok("wrote %s", path)
	return nil
}

// CustomSourcesFilePath returns the path to the generated custom-sources.zsh file.
func CustomSourcesFilePath() string {
	return filepath.Join(core.XDGConfigHome(), "dfinstall", "custom-sources.zsh")
}

// MergeUnique appends additions to existing, deduplicating.
func MergeUnique(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, s := range existing {
		seen[s] = true
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, s := range additions {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}
