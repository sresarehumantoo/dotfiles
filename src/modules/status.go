package modules

import (
	"fmt"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// PrintStatus prints a table showing the status of all modules.
func PrintStatus() {
	fmt.Printf("%-15s  %7s  %7s  %s\n", "MODULE", "LINKED", "MISSING", "INFO")
	fmt.Printf("%-15s  %7s  %7s  %s\n", "------", "------", "-------", "----")

	for _, m := range core.AllModules() {
		s := m.Status()
		fmt.Printf("%-15s  %7d  %7d  %s\n", s.Name, s.Linked, s.Missing, s.Extra)
	}
}

// FormatStatusLine formats a single module status line (exported for testing).
func FormatStatusLine(s core.ModuleStatus) string {
	return fmt.Sprintf("%-15s  %7d  %7d  %s", s.Name, s.Linked, s.Missing, s.Extra)
}
