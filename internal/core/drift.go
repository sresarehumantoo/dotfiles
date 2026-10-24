package core

import (
	"os"
	"sort"
	"strings"
)

// LinkRoot extracts the dotfiles-repo root from a managed symlink target by
// trimming the trailing "/config/..." component. Every managed link points at
// "<root>/config/<...>", so the text before "/config/" is the clone root.
// Returns "" for a target that doesn't look managed (no "/config/" segment).
func LinkRoot(target string) string {
	const marker = "/config/"
	i := strings.Index(target, marker)
	if i < 0 {
		return ""
	}
	return target[:i]
}

// LinkDrift describes how managed symlinks are distributed across dotfiles
// clones on this machine.
type LinkDrift struct {
	// Canonical is the dir DotfilesDir() currently resolves to — where links
	// should point.
	Canonical string
	// Roots maps each clone root that managed symlinks point into to the list
	// of destination paths pointing there.
	Roots map[string][]string
}

// Split reports the drift condition: managed symlinks are spread across more
// than one clone, or they all point at a single clone that isn't the canonical
// one. Either way, `dfinstall install all` from the canonical clone
// consolidates them.
func (d LinkDrift) Split() bool {
	if len(d.Roots) > 1 {
		return true
	}
	for root := range d.Roots {
		if root != d.Canonical {
			return true
		}
	}
	return false
}

// SortedRoots returns the clone roots in stable order for display.
func (d LinkDrift) SortedRoots() []string {
	roots := make([]string, 0, len(d.Roots))
	for r := range d.Roots {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	return roots
}

// DetectLinkDrift inspects every managed symlink (from modules implementing
// LinkExporter) and groups them by the clone root they point into. Missing or
// not-yet-linked targets are ignored — they're a normal "not installed" state,
// not drift. Used by doctor/diff to warn when a host has symlinks split across
// multiple dotfiles clones.
func DetectLinkDrift() LinkDrift {
	d := LinkDrift{Canonical: DotfilesDir(), Roots: map[string][]string{}}
	for _, m := range AllModules() {
		le, ok := m.(LinkExporter)
		if !ok {
			continue
		}
		for _, lp := range le.Links() {
			fi, err := os.Lstat(lp.Dst)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := os.Readlink(lp.Dst)
			if err != nil {
				continue
			}
			if root := LinkRoot(target); root != "" {
				d.Roots[root] = append(d.Roots[root], lp.Dst)
			}
		}
	}
	return d
}
