package modules

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// isToolInstalled checks whether a registry tool is already present on the system.
func isToolInstalled(t core.RegistryTool) bool {
	home, _ := os.UserHomeDir()
	switch t.Method {
	case "appimage":
		_, err := os.Stat(filepath.Join(home, ".local", "bin", t.Binary+".AppImage"))
		return err == nil
	case "git_clone":
		fi, err := os.Stat(filepath.Join(home, ".local", "share", "toolkit", t.Binary))
		return err == nil && fi.IsDir()
	default:
		_, err := exec.LookPath(t.Binary)
		return err == nil
	}
}

// RunToolkitMenu shows an interactive category-based menu for toolkit tools.
// Returns the selected tool names.
func RunToolkitMenu() ([]string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		core.Warn("stdin is not a terminal — skipping toolkit menu")
		return core.Cfg.ToolkitTools, nil
	}

	// Always force refresh when the menu is shown
	reg, err := core.LoadOrFetchRegistry(true)
	if err != nil {
		return nil, fmt.Errorf("fetch toolkit registry: %w", err)
	}

	// Compute installed status for all tools once
	installed := make(map[string]bool, len(reg.Tools))
	for _, t := range reg.Tools {
		if isToolInstalled(t) {
			installed[t.Name] = true
		}
	}

	// Group tools by category, sorted alphabetically (filtered by distro)
	type catGroup struct {
		name  string
		tools []core.RegistryTool
	}
	catIndex := make(map[string]int)
	var cats []catGroup
	for _, t := range reg.Tools {
		if !core.ToolMatchesDistro(t) {
			continue
		}
		idx, ok := catIndex[t.Category]
		if !ok {
			idx = len(cats)
			catIndex[t.Category] = idx
			cats = append(cats, catGroup{name: t.Category})
		}
		cats[idx].tools = append(cats[idx].tools, t)
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].name < cats[j].name })
	for i := range cats {
		sort.Slice(cats[i].tools, func(a, b int) bool {
			return cats[i].tools[a].Name < cats[i].tools[b].Name
		})
	}

	// Pre-select: installed tools + previously configured tools
	selected := make(map[string]bool)
	for name := range installed {
		selected[name] = true
	}
	for _, t := range core.Cfg.ToolkitTools {
		selected[t] = true
	}

	// Persists across iterations so the cursor stays on the last-visited category
	catChoice := ""

	// Category navigation loop
	for {
		// Clear screen between form transitions to prevent leftover renders
		fmt.Print("\033[2J\033[H")

		// Build category options with installed/queued counts
		var catOptions []huh.Option[string]
		for _, cat := range cats {
			catInstalled := 0
			catQueued := 0
			for _, t := range cat.tools {
				if installed[t.Name] {
					catInstalled++
				} else if selected[t.Name] {
					catQueued++
				}
			}
			label := fmt.Sprintf("%s (%d/%d installed)", cat.name, catInstalled, len(cat.tools))
			if catQueued > 0 {
				label = fmt.Sprintf("%s (%d/%d installed · ~%d queued)", cat.name, catInstalled, len(cat.tools), catQueued)
			}
			catOptions = append(catOptions, huh.NewOption(label, cat.name))
		}

		// Compute summary stats for Done label
		var totalInstalled, toInstall, toRemove int
		for name := range selected {
			if installed[name] {
				totalInstalled++
			} else {
				toInstall++
			}
		}
		for name := range installed {
			if !selected[name] {
				toRemove++
			}
		}

		doneLabel := fmt.Sprintf("Done — Installed: %d · To install: %d · To remove: %d", totalInstalled, toInstall, toRemove)
		catOptions = append(catOptions, huh.NewOption(doneLabel, "_done"))

		catForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Toolkit — Select a category").
					Description("Enter to browse, q to quit").
					Options(catOptions...).
					Value(&catChoice),
			),
		).WithKeyMap(escKeyMap())

		if err := catForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				core.PrintHint("Selection cancelled — keeping existing config")
				return core.Cfg.ToolkitTools, nil
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

		// Build tool options — ✓ = installed, ~ = queued (selected but not installed)
		var toolOptions []huh.Option[string]
		var catSelected []string
		for _, t := range cat.tools {
			indicator := "  "
			if installed[t.Name] {
				indicator = "✓ "
			} else if selected[t.Name] {
				indicator = "~ "
			}
			label := fmt.Sprintf("%s%s — %s", indicator, t.Name, t.Description)
			toolOptions = append(toolOptions, huh.NewOption(label, t.Name))
			if selected[t.Name] {
				catSelected = append(catSelected, t.Name)
			}
		}

		toolForm := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title(fmt.Sprintf("Toolkit — %s", cat.name)).
					Description("Space to toggle, Enter to confirm, Esc to go back").
					Options(toolOptions...).
					Value(&catSelected),
			),
		).WithKeyMap(escKeyMap())

		if err := toolForm.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				// Esc goes back to category menu without changes
				continue
			}
			return nil, fmt.Errorf("tool menu: %w", err)
		}

		// Update selections: clear this category, then add back selected
		for _, t := range cat.tools {
			delete(selected, t.Name)
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

	// Validate tool names
	for _, t := range result {
		if !core.ValidToolName.MatchString(t) {
			return nil, fmt.Errorf("invalid tool name %q — must be alphanumeric with hyphens/underscores", t)
		}
	}

	// Show summary relative to what's installed on the system
	var addNames, removeNames []string
	for _, name := range result {
		if !installed[name] {
			addNames = append(addNames, name)
		}
	}
	for name := range installed {
		if !selected[name] {
			removeNames = append(removeNames, name)
		}
	}
	sort.Strings(removeNames)

	keepCount := 0
	for _, name := range result {
		if installed[name] {
			keepCount++
		}
	}

	core.Status("Tools installed: %d", keepCount)
	if len(addNames) > 0 {
		core.Status("Tools to install: %d (%s)", len(addNames), strings.Join(addNames, ", "))
	}
	if len(removeNames) > 0 {
		core.Status("Tools to remove: %d (%s)", len(removeNames), strings.Join(removeNames, ", "))
	}
	if len(addNames) == 0 && len(removeNames) == 0 {
		core.PrintHint("No changes")
	}

	return result, nil
}
