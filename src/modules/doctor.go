package modules

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// DoctorResult is one environment health-check outcome.
type DoctorResult struct {
	Name   string
	OK     bool
	Detail string   // problem summary when !OK, or extra info on an OK check
	Extra  []string // additional indented lines (collision list, drift breakdown)
}

// RunDoctorChecks runs every health check and returns structured results. It is
// the single source of truth shared by the CLI `doctor` command (RunDoctor) and
// the MCP dfinstall_doctor tool, so the two renderings can't drift.
func RunDoctorChecks() []DoctorResult {
	checks := []struct {
		name  string
		check func() string // "" = ok, non-empty = problem
	}{
		{"locale", checkLocale()},
		{"go", checkCommand("go")},
		{"nvim", checkCommand("nvim")},
		{"zsh", checkCommand("zsh")},
		{"tmux", checkCommand("tmux")},
		{"git", checkCommand("git")},
		{"delta", checkCommand("delta")},
		{"curl", checkCommand("curl")},
		{"fzf", checkCommand("fzf")},
		{"ripgrep", checkCommand("rg")},
		{"docker", checkCommand("docker")},
		{"terraform", checkCommand("terraform")},
		{"pip3", checkCommand("pip3")},
		{"oh-my-zsh", checkDir(homeDir(".oh-my-zsh"))},
		{"zsh-autosuggestions", checkDir(homeDir(".oh-my-zsh", "custom", "plugins", "zsh-autosuggestions"))},
		{"powerlevel10k", checkDir(homeDir(".oh-my-zsh", "custom", "themes", "powerlevel10k"))},
		{"fonts", checkFonts()},
		{"nvim config", checkLink(
			core.ConfigPath("nvim", "init.lua"),
			core.XDGTarget("nvim", "init.lua"),
		)},
		{"shell config", checkLink(
			core.ConfigPath("shell", "zshrc"),
			homeDir(".zshrc"),
		)},
		{"git config", checkLink(
			core.ConfigPath("git", "gitconfig"),
			homeDir(".gitconfig"),
		)},
		{"tmux config", checkLink(
			core.ConfigPath("tmux", "tmux.conf"),
			core.XDGTarget("tmux", "tmux.conf"),
		)},
	}

	if len(core.Cfg.ExtendedPlugins) > 0 {
		checks = append(checks, struct {
			name  string
			check func() string
		}{"extended plugins", checkFile(ExtendedPluginsFilePath())})
	}

	if core.IsSteamOS() {
		checks = append(checks,
			struct {
				name  string
				check func() string
			}{"steamos-readonly", checkCommand("steamos-readonly")},
			struct {
				name  string
				check func() string
			}{"pacman", checkCommand("pacman")},
		)
	}

	if core.IsWSL() {
		checks = append(checks,
			struct {
				name  string
				check func() string
				// ⚠ Not checkFileMatch: the wsl.conf template holds @DEFAULT_USER@
				// and never matches the installed file byte-for-byte. Derive from
				// the module's own renderer instead of restating the comparison —
				// same lesson as checkFonts below.
			}{"wsl.conf", checkWslConf()},
			struct {
				name  string
				check func() string
			}{"sysctl config", checkFileMatch(
				core.ConfigPath("wsl", "99-wsl-sysctl.conf"),
				"/etc/sysctl.d/99-wsl.conf",
			)},
			struct {
				name  string
				check func() string
			}{"windows home symlink", checkWinHomeLink()},
		)
	}

	results := make([]DoctorResult, 0, len(checks)+2)
	for _, c := range checks {
		msg := c.check()
		results = append(results, DoctorResult{Name: c.name, OK: msg == "", Detail: msg})
	}

	// Alias collisions between managed and preserved shell files.
	if len(core.Cfg.PreservedFiles) > 0 {
		if collisions := CheckAliasCollisions(); len(collisions) > 0 {
			extra := make([]string, 0, len(collisions)+1)
			for _, c := range collisions {
				extra = append(extra, fmt.Sprintf("%q defined in both ~/.aliases and ~/%s", c.Name, c.PreservedFile))
			}
			extra = append(extra, "remove duplicates from preserved files, or dismiss via: dfinstall install shell")
			results = append(results, DoctorResult{Name: "alias collisions", OK: false, Detail: "preserved files override managed aliases", Extra: extra})
		} else {
			results = append(results, DoctorResult{Name: "alias collisions", OK: true, Detail: "none"})
		}
	}

	// Managed symlinks should all point at one (canonical) dotfiles clone.
	if d := core.DetectLinkDrift(); d.Split() {
		extra := make([]string, 0, len(d.Roots)+1)
		for _, root := range d.SortedRoots() {
			marker := ""
			if root == d.Canonical {
				marker = " (canonical)"
			}
			extra = append(extra, fmt.Sprintf("%s%s — %d link(s)", root, marker, len(d.Roots[root])))
		}
		extra = append(extra, fmt.Sprintf("run 'dfinstall install all' from %s to consolidate", d.Canonical))
		results = append(results, DoctorResult{Name: "dotfiles clones", OK: false, Detail: fmt.Sprintf("symlinks split across %d location(s)", len(d.Roots)), Extra: extra})
	} else {
		results = append(results, DoctorResult{Name: "dotfiles clones", OK: true, Detail: "single source"})
	}

	return results
}

// RunDoctor prints the health-check results for the CLI `doctor` command.
func RunDoctor() {
	fmt.Println("Running health checks...")
	fmt.Println()

	// ⚠ core.Status/AlwaysWarn, NOT core.Ok/core.Warn. Printing the results IS
	// this command's entire purpose, so its output must not respect the log
	// level: Ok is suppressed below LogVerbose and Warn is buffered until a
	// FlushWarnings that RunDoctor never called. The effect was that plain
	// `dfinstall doctor` printed its header and nothing else — every check,
	// pass or fail, silently discarded — and only `doctor -v` worked. Measured
	// before the fix: 10 lines vs 34 with -v, identical under a pty, so this
	// was never a TTY-detection issue.
	allOk := true
	for _, r := range RunDoctorChecks() {
		if r.OK {
			if r.Detail != "" {
				core.Status("%s: %s", r.Name, r.Detail)
			} else {
				core.Status("%s", r.Name)
			}
		} else {
			core.AlwaysWarn("%s — %s", r.Name, r.Detail)
			for _, e := range r.Extra {
				core.AlwaysWarn("    %s", e)
			}
			allOk = false
		}
	}

	fmt.Println()
	if allOk {
		core.Status("All checks passed!")
	} else {
		core.AlwaysWarn("Some checks failed. Run 'dfinstall install all' to fix.")
	}
}

func homeDir(parts ...string) string {
	return core.HomeTarget(parts...)
}

func checkCommand(name string) func() string {
	return func() string {
		if _, err := exec.LookPath(name); err != nil {
			return "not found"
		}
		return ""
	}
}

func checkDir(path string) func() string {
	return func() string {
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			return "not found"
		}
		return ""
	}
}

func checkFile(path string) func() string {
	return func() string {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "not found"
		}
		return ""
	}
}

// checkLink verifies a symlink at dst points to src.
func checkLink(src, dst string) func() string {
	return func() string {
		switch core.CheckLink(src, dst) {
		case "ok":
			return ""
		case "wrong":
			return "wrong target"
		case "file":
			return "regular file (not symlinked)"
		default:
			return "not found"
		}
	}
}

// checkWslConf derives from the wsl module's renderer rather than diffing the
// raw template, which contains an unsubstituted @DEFAULT_USER@ placeholder.
func checkWslConf() func() string {
	return func() string {
		switch wslConfState() {
		case "ok":
			return ""
		case "outdated":
			return "outdated"
		default:
			return "not found"
		}
	}
}

// checkFileMatch verifies dst exists and has identical content to src (by hash).
//
// ⚠ Only valid for sources installed VERBATIM. Anything rendered from a
// template (wsl.conf) must derive from the module's own renderer instead —
// hashing the template would never match and would report permanent drift.
func checkFileMatch(src, dst string) func() string {
	return func() string {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return "not found"
		}
		if !core.FilesMatch(src, dst) {
			return "outdated"
		}
		return ""
	}
}

// checkFonts derives entirely from the fonts module — its Links() for the
// vendored floor, and fontNotes() for the downloaded families — rather than
// restating any path or filename of its own.
//
// It used to name a file directly (checkFontMatch("HackNerdFont-Regular.ttf"),
// under a hardcoded ~/.local/share/fonts), and the Hack -> IosevkaTerm migration
// walked straight past it. The result was inverted: a correctly migrated machine
// reported "fonts — not found" and was told to run `install all`, which could
// never fix it, while a machine still littered with stale Hack files passed. A
// check that restates what a module does will eventually disagree with it, so
// this one asks the module.
func checkFonts() func() string {
	return func() string {
		m := FontsModule{}
		var problems []string
		for _, l := range m.Links() {
			if core.CheckLink(l.Src, l.Dst) != "ok" {
				problems = append(problems, filepath.Base(l.Dst)+" not linked")
			}
		}
		problems = append(problems, fontNotes()...)
		return strings.Join(problems, ", ")
	}
}

func checkLocale() func() string {
	return func() string {
		if localeGenerated("en_US.UTF-8") {
			return ""
		}
		return "en_US.UTF-8 not generated (run: dfinstall install locale)"
	}
}

func checkWinHomeLink() func() string {
	return func() string {
		wslWinHome := resolveWinHome()
		if wslWinHome == "" {
			return "could not resolve Windows home"
		}
		winUser := filepath.Base(wslWinHome)
		switch core.CheckLink(wslWinHome, core.HomeTarget(winUser)) {
		case "ok":
			return ""
		case "wrong":
			return "wrong target"
		case "file":
			return "regular file (not symlinked)"
		default:
			return "not found"
		}
	}
}
