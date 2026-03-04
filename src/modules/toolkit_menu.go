package modules

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// RunToolkitMenu shows an interactive multi-select for toolkit tools.
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

	// Build options from registry
	var options []huh.Option[string]
	for _, t := range reg.Tools {
		label := fmt.Sprintf("%s — %s", t.Name, t.Description)
		options = append(options, huh.NewOption(label, t.Name))
	}

	// Pre-select from saved config
	previous := make(map[string]bool, len(core.Cfg.ToolkitTools))
	for _, t := range core.Cfg.ToolkitTools {
		previous[t] = true
	}

	selected := make([]string, len(core.Cfg.ToolkitTools))
	copy(selected, core.Cfg.ToolkitTools)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Toolkit — Security, CTF & Dev Tools").
				Description("Space to toggle, Enter to confirm, Esc to cancel").
				Options(options...).
				Value(&selected),
		),
	).WithKeyMap(escKeyMap())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			core.PrintHint("Selection cancelled — keeping existing config")
			return core.Cfg.ToolkitTools, nil
		}
		return nil, fmt.Errorf("toolkit menu: %w", err)
	}

	// Validate tool names
	for _, t := range selected {
		if !core.ValidToolName.MatchString(t) {
			return nil, fmt.Errorf("invalid tool name %q — must be alphanumeric with hyphens/underscores", t)
		}
	}

	// Show feedback about what changed
	newSet := make(map[string]bool, len(selected))
	for _, t := range selected {
		newSet[t] = true
	}

	var added, removed []string
	for _, t := range selected {
		if !previous[t] {
			added = append(added, t)
		}
	}
	for t := range previous {
		if !newSet[t] {
			removed = append(removed, t)
		}
	}

	if len(added) == 0 && len(removed) == 0 {
		if len(selected) > 0 {
			core.PrintHint(fmt.Sprintf("Toolkit unchanged (%d tools selected)", len(selected)))
		} else {
			core.PrintHint("No toolkit tools selected")
		}
	} else {
		if len(added) > 0 {
			core.Status("Enabling: %s", strings.Join(added, ", "))
		}
		if len(removed) > 0 {
			core.Status("Disabling: %s", strings.Join(removed, ", "))
		}
		core.Status("Toolkit: %d tools selected", len(selected))
	}

	return selected, nil
}
