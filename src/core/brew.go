package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Which Homebrew to use is discovered, not assumed. A managed Mac can have an
// IT-owned /opt/homebrew this user cannot write to beside one the user owns,
// so the choice follows ~/.profile's order exactly: the prefix the macOS
// bootstrap recorded, else the first candidate this user can write to, else
// the first that exists (a read-only Homebrew still runs what it holds).

const wOK = 0x2 // access(2) W_OK

// Writable reports whether this process may write to path.
func Writable(path string) bool {
	return path != "" && syscall.Access(path, wOK) == nil
}

// BrewWritable reports whether this user can install with the Homebrew at
// prefix. Homebrew supports one owning account; anyone else gets permission
// errors from every install.
func BrewWritable(prefix string) bool {
	return Writable(filepath.Join(prefix, "Cellar")) || Writable(prefix)
}

// BrewPrefixFile is where the macOS bootstrap records the prefix it settled on.
func BrewPrefixFile() string {
	return XDGTarget("dfinstall", "brew-prefix")
}

// BrewPrefix returns the Homebrew prefix to use, or "" when there is none.
func BrewPrefix() string {
	var recorded string
	if data, err := os.ReadFile(BrewPrefixFile()); err == nil {
		recorded = strings.TrimSpace(string(data))
	}
	candidates := []string{os.Getenv("HOMEBREW_PREFIX")}
	if brew, err := exec.LookPath("brew"); err == nil {
		candidates = append(candidates, filepath.Dir(filepath.Dir(brew)))
	}
	candidates = append(candidates, "/opt/homebrew", "/usr/local", HomeTarget("homebrew"))
	return brewPrefixFrom(recorded, candidates)
}

func brewPrefixFrom(recorded string, candidates []string) string {
	has := func(p string) bool {
		if p == "" {
			return false // $HOME unset; a relative path would probe the CWD
		}
		st, err := os.Stat(filepath.Join(p, "bin", "brew"))
		return err == nil && !st.IsDir()
	}
	if has(recorded) {
		return recorded
	}
	fallback := ""
	for _, p := range candidates {
		if !has(p) {
			continue
		}
		if BrewWritable(p) {
			return p
		}
		if fallback == "" {
			fallback = p
		}
	}
	return fallback
}
