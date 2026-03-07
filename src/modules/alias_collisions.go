package modules

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

var (
	aliasRe = regexp.MustCompile(`^\s*alias\s+([a-zA-Z0-9_-]+)=`)
	funcRe  = regexp.MustCompile(`^\s*function\s+([a-zA-Z0-9_-]+)`)
	funcRe2 = regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+)\s*\(\)`)
)

// shellNames extracts alias and function names from a shell file.
func shellNames(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, re := range []*regexp.Regexp{aliasRe, funcRe, funcRe2} {
			if m := re.FindStringSubmatch(line); m != nil {
				names[m[1]] = true
			}
		}
	}
	return names, scanner.Err()
}

// AliasCollision represents a naming conflict between managed and preserved files.
type AliasCollision struct {
	Name         string
	PreservedFile string
}

// CheckAliasCollisions scans preserved files for aliases/functions that collide
// with our managed aliases file.
func CheckAliasCollisions() []AliasCollision {
	managedPath := core.ConfigPath("shell", "aliases")
	managed, err := shellNames(managedPath)
	if err != nil {
		core.Debug("alias collision check: could not read managed aliases: %v", err)
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var collisions []AliasCollision
	for _, p := range core.Cfg.PreservedFiles {
		fullPath := fmt.Sprintf("%s/%s", home, p)
		names, err := shellNames(fullPath)
		if err != nil {
			continue
		}
		for name := range names {
			if managed[name] {
				collisions = append(collisions, AliasCollision{
					Name:         name,
					PreservedFile: p,
				})
			}
		}
	}
	return collisions
}

// ReportAliasCollisions warns about any detected collisions.
// Returns true if collisions were found.
func ReportAliasCollisions() bool {
	collisions := CheckAliasCollisions()
	if len(collisions) == 0 {
		return false
	}

	core.Warn("Alias/function collisions detected (preserved files override managed aliases):")
	for _, c := range collisions {
		core.Warn("  %q defined in both ~/.aliases and ~/%s", c.Name, c.PreservedFile)
	}
	core.PrintHint("Remove duplicates from preserved files, or dismiss the file via: dfinstall install shell")
	return true
}
