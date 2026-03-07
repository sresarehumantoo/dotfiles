package modules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// validPluginName matches safe OMZ plugin names (alphanumeric, hyphens, underscores).
var validPluginName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

type pluginOption struct {
	Name     string
	Desc     string
	Category string
}

var extendedPluginOptions = []pluginOption{
	// Container & Orchestration
	{"kubectl", "Kubernetes CLI completions & aliases", "Container & Orchestration"},
	{"helm", "Helm completions", "Container & Orchestration"},
	{"docker-compose", "Docker Compose completions & aliases", "Container & Orchestration"},

	// Cloud
	{"aws", "AWS CLI completions", "Cloud"},
	{"gcloud", "Google Cloud SDK completions", "Cloud"},
	{"azure", "Azure CLI completions", "Cloud"},

	// Languages & Tools
	{"npm", "npm completions & aliases", "Languages & Tools"},
	{"yarn", "Yarn completions & aliases", "Languages & Tools"},
	{"pip", "pip completions", "Languages & Tools"},
	{"rust", "Rust/Cargo completions", "Languages & Tools"},
	{"python", "Python aliases & helpers", "Languages & Tools"},
	{"ruby", "Ruby aliases & helpers", "Languages & Tools"},
	{"dotnet", ".NET CLI completions", "Languages & Tools"},

	// DevOps
	{"ansible", "Ansible completions & aliases", "DevOps"},
	{"vagrant", "Vagrant completions & aliases", "DevOps"},

	// Utilities
	{"sudo", "Double-ESC to prefix sudo", "Utilities"},
	{"rsync", "Rsync aliases", "Utilities"},
	{"systemd", "Systemd aliases", "Utilities"},
	{"encode64", "Base64 encode/decode helpers", "Utilities"},
	{"jsontools", "JSON formatting helpers", "Utilities"},
	{"urltools", "URL encode/decode helpers", "Utilities"},
	{"command-not-found", "Suggest packages for unknown commands", "Utilities"},
}

// isOmzPluginAvailable checks whether an OMZ plugin directory exists.
func isOmzPluginAvailable(name string) bool {
	home, _ := os.UserHomeDir()
	bundled := filepath.Join(home, ".oh-my-zsh", "plugins", name)
	if fi, err := os.Stat(bundled); err == nil && fi.IsDir() {
		return true
	}
	custom := filepath.Join(home, ".oh-my-zsh", "custom", "plugins", name)
	if fi, err := os.Stat(custom); err == nil && fi.IsDir() {
		return true
	}
	return false
}

// RunExtendedPluginMenu shows an interactive category-based menu for extended OMZ plugins.
// Returns the selected plugin names.
func RunExtendedPluginMenu() ([]string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		core.Warn("stdin is not a terminal — skipping extended plugin menu")
		return core.Cfg.ExtendedPlugins, nil
	}

	// Group plugins by category, sorted alphabetically
	type catGroup struct {
		name    string
		plugins []pluginOption
	}
	var cats []catGroup
	catIndex := make(map[string]int)
	for _, p := range extendedPluginOptions {
		idx, ok := catIndex[p.Category]
		if !ok {
			idx = len(cats)
			catIndex[p.Category] = idx
			cats = append(cats, catGroup{name: p.Category})
		}
		cats[idx].plugins = append(cats[idx].plugins, p)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].name < cats[j].name })
	for i := range cats {
		sort.Slice(cats[i].plugins, func(a, b int) bool {
			return cats[i].plugins[a].Name < cats[i].plugins[b].Name
		})
	}

	// Check which plugins are available (OMZ dir exists)
	available := make(map[string]bool)
	for _, p := range extendedPluginOptions {
		if isOmzPluginAvailable(p.Name) {
			available[p.Name] = true
		}
	}

	// Pre-select from saved config
	selected := make(map[string]bool, len(core.Cfg.ExtendedPlugins))
	for _, p := range core.Cfg.ExtendedPlugins {
		selected[p] = true
	}

	// Persists across iterations so the cursor stays on the last-visited category
	catChoice := ""

	// Category navigation loop
	for {
		// Clear screen between form transitions
		fmt.Print("\033[2J\033[H")

		// Build category options with enabled counts
		var catOptions []huh.Option[string]
		for _, cat := range cats {
			count := 0
			for _, p := range cat.plugins {
				if selected[p.Name] {
					count++
				}
			}
			label := fmt.Sprintf("%s (%d/%d enabled)", cat.name, count, len(cat.plugins))
			catOptions = append(catOptions, huh.NewOption(label, cat.name))
		}

		total := 0
		for range selected {
			total++
		}
		doneLabel := fmt.Sprintf("Done (%d plugins enabled)", total)
		catOptions = append(catOptions, huh.NewOption(doneLabel, "_done"))

		catForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Extended OMZ Plugins — Select a category").
					Description("Enter to browse, q to quit").
					Options(catOptions...).
					Value(&catChoice),
			),
		).WithKeyMap(escKeyMap())

		if err := catForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				core.PrintHint("Selection cancelled — keeping existing config")
				return core.Cfg.ExtendedPlugins, nil
			}
			return nil, fmt.Errorf("category menu: %w", err)
		}

		if catChoice == "_done" {
			break
		}

		// Find the selected category
		var cat *catGroup
		for i := range cats {
			if cats[i].name == catChoice {
				cat = &cats[i]
				break
			}
		}
		if cat == nil {
			continue
		}

		// Build plugin options with availability indicator
		var pluginOptions []huh.Option[string]
		var catSelected []string
		for _, p := range cat.plugins {
			indicator := "  "
			if available[p.Name] {
				indicator = "✓ "
			}
			label := fmt.Sprintf("%s%s — %s", indicator, p.Name, p.Desc)
			pluginOptions = append(pluginOptions, huh.NewOption(label, p.Name))
			if selected[p.Name] {
				catSelected = append(catSelected, p.Name)
			}
		}

		pluginForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(fmt.Sprintf("OMZ Plugins — %s", cat.name)).
					Description("Space to toggle, Enter to confirm, Esc to go back").
					Options(pluginOptions...).
					Value(&catSelected),
			),
		).WithKeyMap(escKeyMap())

		if err := pluginForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				continue
			}
			return nil, fmt.Errorf("plugin menu: %w", err)
		}

		// Update selections: clear this category, add back selected
		for _, p := range cat.plugins {
			delete(selected, p.Name)
		}
		for _, name := range catSelected {
			selected[name] = true
		}
	}

	// Convert map to sorted slice
	var result []string
	for name := range selected {
		result = append(result, name)
	}
	sort.Strings(result)

	// Validate plugin names
	for _, p := range result {
		if !validPluginName.MatchString(p) {
			return nil, fmt.Errorf("invalid plugin name %q — must be alphanumeric with hyphens/underscores", p)
		}
	}

	// Show feedback about what changed
	previous := make(map[string]bool, len(core.Cfg.ExtendedPlugins))
	for _, p := range core.Cfg.ExtendedPlugins {
		previous[p] = true
	}

	var added, removed []string
	for _, p := range result {
		if !previous[p] {
			added = append(added, p)
		}
	}
	for p := range previous {
		if !selected[p] {
			removed = append(removed, p)
		}
	}

	if len(added) == 0 && len(removed) == 0 {
		if len(result) > 0 {
			core.PrintHint(fmt.Sprintf("Extended plugins unchanged (%d enabled)", len(result)))
		} else {
			core.PrintHint("No extended plugins selected")
		}
	} else {
		if len(added) > 0 {
			core.Status("Enabling: %s", strings.Join(added, ", "))
		}
		if len(removed) > 0 {
			core.Status("Disabling: %s", strings.Join(removed, ", "))
		}
		core.Status("Extended plugins: %d enabled", len(result))
		core.PrintHint("Changes take effect after: exec zsh")
	}

	return result, nil
}

// WriteExtendedPluginsFile writes the generated plugins.zsh sourced by zshrc.
func WriteExtendedPluginsFile(plugins []string) error {
	dir := filepath.Join(core.XDGConfigHome(), "dfinstall")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dfinstall config dir: %w", err)
	}

	path := filepath.Join(dir, "plugins.zsh")

	// Validate plugin names before writing to a shell-sourced file
	for _, p := range plugins {
		if !validPluginName.MatchString(p) {
			return fmt.Errorf("invalid plugin name %q — must be alphanumeric with hyphens/underscores", p)
		}
	}

	var sb strings.Builder
	sb.WriteString("# Generated by dfinstall — do not edit manually.\n")
	sb.WriteString("# Re-run: dfinstall install omz --extended\n\n")

	if len(plugins) > 0 {
		sb.WriteString("DFINSTALL_EXTENDED_PLUGINS=(\n")
		for _, p := range plugins {
			sb.WriteString("  " + p + "\n")
		}
		sb.WriteString(")\n")
	} else {
		sb.WriteString("DFINSTALL_EXTENDED_PLUGINS=()\n")
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write plugins.zsh: %w", err)
	}

	core.Ok("wrote %s", path)
	return nil
}

// ExtendedPluginsFilePath returns the path to the generated plugins.zsh file.
func ExtendedPluginsFilePath() string {
	return filepath.Join(core.XDGConfigHome(), "dfinstall", "plugins.zsh")
}
