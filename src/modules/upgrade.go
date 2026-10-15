package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"golang.org/x/term"
)

// updateClone fast-forwards a plugin or theme dfinstall cloned earlier. One
// with local commits, or edits in the files upstream changed, fails the
// fast-forward and is left as it is.
func updateClone(ctx context.Context, name, dir string) {
	if !core.UpgradeMode {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return
	}
	if err := runNetCmd(ctx, "git", "-C", dir, "pull", "--ff-only", "-q"); err != nil {
		core.Warn("%s not updated: %v", name, err)
		return
	}
	core.Ok("%s updated", name)
}

// ConvergeClone brings the dotfiles clone at dir up to date for --upgrade,
// keeping what was changed on this machine:
//
//   - a managed path some program replaced with a real file is offered for
//     adoption into the clone, rather than going to .bak unseen
//   - an edit to a managed file that has a machine-local override file is
//     offered a move there, which leaves the clone clean
//   - the clone is fast-forwarded to its upstream, carrying remaining edits
//     across, but only when they merge cleanly with what upstream changed
//
// Local commits, an operation in progress, or an edit that would conflict
// stop the pull; the rest of the install still runs. Reports whether
// dfinstall's own source changed, so the caller can rebuild before installing.
func ConvergeClone(ctx context.Context, dir string) (srcChanged bool, err error) {
	if _, err := gitOut(ctx, dir, "rev-parse", "--git-dir"); err != nil {
		core.Notice("%s is not a git checkout; nothing to pull", dir)
		return false, nil
	}
	if op := gitOperation(ctx, dir); op != "" {
		core.AlwaysWarn("%s has a %s in progress; finish it, then rerun --upgrade", dir, op)
		return false, nil
	}

	rescueReplacedLinks(ctx, dir)
	moveLocalEdits(ctx, dir)

	upstream, err := gitOut(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		core.AlwaysWarn("%s has no upstream branch; not pulling", dir)
		return false, nil
	}
	if core.DryRun {
		core.Info("would fetch %s (counts below are as of the last fetch)", upstream)
	} else if err := runNetCmd(ctx, "git", "-C", dir, "fetch", "--quiet"); err != nil {
		core.AlwaysWarn("could not fetch %s: %v", upstream, err)
		return false, nil
	}

	counts, err := gitOut(ctx, dir, "rev-list", "--left-right", "--count", "@{u}...HEAD")
	if err != nil {
		return false, err
	}
	behind, ahead, ok := parseAheadBehind(counts)
	switch {
	case !ok:
		return false, fmt.Errorf("could not compare with %s", upstream)
	case behind == 0:
		core.Status("dotfiles clone is up to date with %s", upstream)
		return false, nil
	case ahead > 0:
		core.AlwaysWarn("%s has %d commit(s) not on %s; not pulling. Rebase or merge it yourself, then rerun", dir, ahead, upstream)
		return false, nil
	}

	edited, err := editedFiles(ctx, dir)
	if err != nil {
		return false, err
	}
	changed, err := gitLines(ctx, dir, "diff", "--name-only", "HEAD", "@{u}")
	if err != nil {
		return false, err
	}
	var conflicts []string
	for _, f := range edited {
		if slices.Contains(changed, f) && !mergesCleanly(ctx, dir, f) {
			conflicts = append(conflicts, f)
		}
	}
	if len(conflicts) > 0 {
		core.AlwaysWarn("not pulling %d commit(s) from %s: your edits to %s conflict with upstream's. Commit, move or revert them, then rerun",
			behind, upstream, strings.Join(conflicts, ", "))
		return false, nil
	}
	if core.DryRun {
		core.Info("would fast-forward %d commit(s) from %s", behind, upstream)
		return false, nil
	}

	before, err := gitOut(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	if err := fastForward(ctx, dir, len(edited) > 0); err != nil {
		return false, err
	}
	srcFiles, err := gitLines(ctx, dir, "diff", "--name-only", before, "HEAD", "--", "src", "go.mod", "go.sum", "Makefile")
	if err != nil {
		return false, err
	}
	core.Status("dotfiles clone fast-forwarded %d commit(s) from %s", behind, upstream)
	return len(srcFiles) > 0, nil
}

// fastForward merges the upstream, setting edits aside around it. The caller
// has already checked that they apply cleanly afterwards.
func fastForward(ctx context.Context, dir string, stash bool) error {
	if !stash {
		return runCmd(ctx, "git", "-C", dir, "merge", "--ff-only", "--quiet", "@{u}")
	}
	if err := runCmd(ctx, "git", "-C", dir, "stash", "push", "--quiet", "-m", "dfinstall install --upgrade"); err != nil {
		return err
	}
	mergeErr := runCmd(ctx, "git", "-C", dir, "merge", "--ff-only", "--quiet", "@{u}")
	if err := runCmd(ctx, "git", "-C", dir, "stash", "pop", "--quiet"); err != nil {
		core.AlwaysWarn("your edits could not be reapplied; they are kept in `git -C %s stash list`", dir)
		return errors.Join(mergeErr, err)
	}
	return mergeErr
}

// mergesCleanly reports whether the worktree's edits to f survive upstream's
// change to it without a conflict, by doing that three-way merge in memory.
func mergesCleanly(ctx context.Context, dir, f string) bool {
	tmp, err := os.MkdirTemp("", "dfinstall-merge-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmp)
	base, theirs := filepath.Join(tmp, "base"), filepath.Join(tmp, "theirs")
	for rev, dst := range map[string]string{"HEAD": base, "@{u}": theirs} {
		out, err := runProbe(ctx, "git", "-C", dir, "show", rev+":"+f)
		if err != nil {
			return false // added or deleted on one side: not a plain edit
		}
		if err := os.WriteFile(dst, out, 0o600); err != nil {
			return false
		}
	}
	_, err = runProbe(ctx, "git", "merge-file", "-p", filepath.Join(dir, f), base, theirs)
	return err == nil
}

// gitOperation names a rebase, merge or cherry-pick left in progress in dir.
func gitOperation(ctx context.Context, dir string) string {
	for marker, name := range map[string]string{
		"rebase-merge": "rebase", "rebase-apply": "rebase", "MERGE_HEAD": "merge",
		"CHERRY_PICK_HEAD": "cherry-pick", "REVERT_HEAD": "revert",
	} {
		p, err := gitOut(ctx, dir, "rev-parse", "--path-format=absolute", "--git-path", marker)
		if err != nil {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return name
		}
	}
	return ""
}

// editedFiles lists tracked files in dir that differ from HEAD, staged or not.
func editedFiles(ctx context.Context, dir string) ([]string, error) {
	return gitLines(ctx, dir, "diff", "--name-only", "HEAD")
}

func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := runProbe(ctx, "git", append([]string{"-C", dir}, args...)...)
	return strings.TrimSpace(string(out)), err
}

func gitLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := gitOut(ctx, dir, args...)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

func canPrompt() bool {
	return core.Interactive && term.IsTerminal(int(os.Stdin.Fd()))
}

// rescueReplacedLinks finds managed paths that are no longer symlinks and
// differ from the clone's copy. Linking would move them to .bak, so a change
// made there would quietly stop applying; adopting copies it into the clone,
// where moveLocalEdits and the pull treat it like any other edit.
func rescueReplacedLinks(ctx context.Context, dir string) {
	root := core.DotfilesDir()
	for _, m := range core.AllModules() {
		le, ok := m.(core.LinkExporter)
		if !ok || core.SkipInAll(m.Name()) {
			continue
		}
		for _, l := range le.Links() {
			fi, err := os.Lstat(l.Dst)
			if err != nil || !fi.Mode().IsRegular() {
				continue
			}
			rel, err := filepath.Rel(root, l.Src)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
			src := filepath.Join(dir, rel)
			theirs, err1 := os.ReadFile(l.Dst)
			ours, err2 := os.ReadFile(src)
			if err1 != nil || err2 != nil || bytes.Equal(theirs, ours) {
				continue
			}
			adoptReplacedLink(ctx, src, l.Dst, theirs)
		}
	}
}

func adoptReplacedLink(ctx context.Context, src, dst string, content []byte) {
	if core.DryRun {
		core.Info("would offer to copy %s, a file where a link belongs, into %s", dst, src)
		return
	}
	if !canPrompt() {
		core.Notice("%s replaced its link and differs from %s; it will be moved to %s.bak. Rerun --upgrade in a terminal to keep it", dst, src, dst)
		return
	}
	diff, _ := runProbe(ctx, "git", "diff", "--no-index", "--no-color", "--", src, dst)
	keep := true
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s is a file, not a link, and differs from the clone", dst)).
			Description(clip(string(diff), 30) + "\nKeep this machine's version? It is copied into the clone, then linked.").
			Affirmative("Keep mine").
			Negative("Use the clone's (mine goes to .bak)").
			Value(&keep),
	))
	if err := form.Run(); err != nil || !keep {
		return
	}
	fi, err := os.Stat(src)
	if err != nil {
		core.AlwaysWarn("could not adopt %s: %v", dst, err)
		return
	}
	if err := os.WriteFile(src, content, fi.Mode().Perm()); err != nil {
		core.AlwaysWarn("could not adopt %s: %v", dst, err)
		return
	}
	core.Status("copied %s into %s", dst, src)
}

// overrideKind is how an edit is carried into a machine-local override file.
type overrideKind int

const (
	// overrideLines appends the added lines; for files sourced in order,
	// where a later line adds to or overrides an earlier one.
	overrideLines overrideKind = iota
	// overrideGhostty is overrideLines where a changed key also moves, since
	// a key set again later wins.
	overrideGhostty
	// overrideGit adds new keys with git config. ~/.gitconfig.local is
	// included at the top of gitconfig, so it can add keys but not override
	// one the main file sets.
	overrideGit
)

type override struct {
	path string
	kind overrideKind
}

// overrideFor maps a file in the clone to the override file that is loaded
// after it on every machine but kept outside the clone.
func overrideFor(rel string) (override, bool) {
	var o override
	switch {
	case rel == "config/ghostty/config":
		o = override{core.XDGTarget("ghostty", "ghostty.user"), overrideGhostty}
	case rel == "config/git/gitconfig":
		o = override{core.HomeTarget(".gitconfig.local"), overrideGit}
	case rel == "config/shell/zshrc", path.Dir(rel) == "config/shell/zsh" && path.Ext(rel) == ".zsh":
		o = override{core.HomeTarget(".zshrc.local"), overrideLines}
	case rel == "config/shell/aliases":
		o = override{core.HomeTarget(".aliases.local"), overrideLines}
	default:
		return o, false
	}
	return o, o.path != ""
}

// ghosttyRepeatable are Ghostty keys where a second assignment adds to the
// list instead of replacing it, so a changed value cannot be moved.
var ghosttyRepeatable = map[string]bool{
	"keybind": true, "palette": true, "custom-shader": true, "config-file": true,
	"font-family": true, "font-family-bold": true, "font-family-italic": true,
	"font-family-bold-italic": true, "font-feature": true, "font-variation": true,
	"font-variation-bold": true, "font-variation-italic": true,
	"font-variation-bold-italic": true, "font-codepoint-map": true, "env": true,
}

// movedEdit is what moving one file's edits would write: lines to append, or
// for git, key/value pairs to add.
type movedEdit struct {
	lines   []string
	entries [][2]string
}

func (e movedEdit) empty() bool { return len(e.lines) == 0 && len(e.entries) == 0 }

func (e movedEdit) String() string {
	if len(e.entries) == 0 {
		return strings.Join(e.lines, "\n")
	}
	var b strings.Builder
	for _, kv := range e.entries {
		fmt.Fprintf(&b, "%s = %s\n", kv[0], kv[1])
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// planMove works out whether an edit can be expressed in its override file.
// It cannot when it removes or changes something the override, loaded later,
// has no way to undo.
func planMove(ctx context.Context, dir, rel string, kind overrideKind) (movedEdit, bool) {
	if kind == overrideGit {
		return planGitMove(ctx, dir, rel)
	}
	out, err := runProbe(ctx, "git", "-C", dir, "diff", "--no-color", "--no-ext-diff", "-U0", "HEAD", "--", rel)
	if err != nil {
		return movedEdit{}, false
	}
	added, removed := splitDiff(string(out))
	addedKeys := map[string]bool{}
	for _, l := range added {
		if k := ghosttyKey(l); k != "" {
			addedKeys[k] = true
		}
	}
	for _, l := range removed {
		if inert(l) {
			continue
		}
		k := ghosttyKey(l)
		if kind != overrideGhostty || k == "" || !addedKeys[k] || ghosttyRepeatable[k] {
			return movedEdit{}, false
		}
	}
	return movedEdit{lines: trimBlank(added)}, true
}

// splitDiff returns the added and removed lines of a unified diff.
func splitDiff(diff string) (added, removed []string) {
	inHunk := false
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "@@"):
			inHunk = true
		case !inHunk:
		case strings.HasPrefix(l, "+"):
			added = append(added, l[1:])
		case strings.HasPrefix(l, "-"):
			removed = append(removed, l[1:])
		}
	}
	return added, removed
}

func inert(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}

func ghosttyKey(line string) string {
	if inert(line) {
		return ""
	}
	k, _, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(k)
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// planGitMove compares the parsed config rather than its lines, so moving
// keys between sections or reformatting does not matter.
func planGitMove(ctx context.Context, dir, rel string) (movedEdit, bool) {
	head, err := runProbe(ctx, "git", "-C", dir, "show", "HEAD:"+rel)
	if err != nil {
		return movedEdit{}, false
	}
	tmp, err := os.CreateTemp("", "dfinstall-gitconfig-*")
	if err != nil {
		return movedEdit{}, false
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(head); err != nil || tmp.Close() != nil {
		return movedEdit{}, false
	}
	before, err1 := gitConfigEntries(ctx, tmp.Name())
	after, err2 := gitConfigEntries(ctx, filepath.Join(dir, rel))
	if err1 != nil || err2 != nil {
		return movedEdit{}, false
	}
	keys := map[string]bool{}
	remaining := slices.Clone(after)
	for _, kv := range before {
		keys[kv[0]] = true
		i := slices.Index(remaining, kv)
		if i < 0 {
			return movedEdit{}, false // removed or changed
		}
		remaining = slices.Delete(remaining, i, i+1)
	}
	for _, kv := range remaining {
		if keys[kv[0]] {
			return movedEdit{}, false // the main file's value would still win
		}
	}
	return movedEdit{entries: remaining}, true
}

func gitConfigEntries(ctx context.Context, file string) ([][2]string, error) {
	out, err := runProbe(ctx, "git", "config", "--file", file, "--list", "-z")
	if err != nil {
		return nil, err
	}
	var entries [][2]string
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		k, v, _ := strings.Cut(rec, "\n")
		entries = append(entries, [2]string{k, v})
	}
	return entries, nil
}

// moveLocalEdits offers to move each edited managed file's changes into its
// override file. Accepted moves revert the file in the clone; anything else
// stays a local edit for the pull to carry.
func moveLocalEdits(ctx context.Context, dir string) {
	edited, err := editedFiles(ctx, dir)
	if err != nil {
		return
	}
	for _, rel := range edited {
		o, ok := overrideFor(rel)
		if !ok {
			core.Notice("%s has local edits with no override file to move them to; kept in the clone", rel)
			continue
		}
		edit, ok := planMove(ctx, dir, rel, o.kind)
		if !ok {
			core.Notice("%s has local edits that change or remove upstream lines, which %s cannot undo; kept in the clone", rel, o.path)
			continue
		}
		if edit.empty() {
			continue
		}
		switch {
		case core.DryRun:
			core.Info("would offer to move the edits to %s into %s:\n%s", rel, o.path, edit)
			continue
		case !canPrompt():
			core.Notice("%s has local edits that could move to %s; rerun --upgrade in a terminal to move them", rel, o.path)
			continue
		}
		move := true
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Move this machine's edits to %s into %s?", rel, o.path)).
				Description(clip(edit.String(), 30) + "\n\nThey keep applying from there, and the clone goes back to upstream's version.").
				Affirmative("Move them").
				Negative("Keep them in the clone").
				Value(&move),
		))
		if err := form.Run(); err != nil || !move {
			continue
		}
		if err := applyMove(ctx, o, rel, edit); err != nil {
			core.AlwaysWarn("could not move edits to %s: %v", rel, err)
			continue
		}
		if err := runCmd(ctx, "git", "-C", dir, "checkout", "HEAD", "--", rel); err != nil {
			core.AlwaysWarn("moved edits to %s into %s, but could not revert it in the clone: %v", rel, o.path, err)
			continue
		}
		core.Status("moved edits to %s into %s", rel, o.path)
	}
}

func applyMove(ctx context.Context, o override, rel string, edit movedEdit) error {
	if err := core.CheckTarget(o.path); err != nil {
		return err
	}
	if o.kind == overrideGit {
		for _, kv := range edit.entries {
			if err := runCmd(ctx, "git", "config", "--file", o.path, "--add", kv[0], kv[1]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(o.path), 0o755); err != nil {
		return err
	}
	old, err := os.ReadFile(o.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var b strings.Builder
	b.Write(old)
	if len(old) > 0 {
		if !bytes.HasSuffix(old, []byte("\n")) {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "# moved from %s by dfinstall install --upgrade\n%s\n", rel, edit)
	return os.WriteFile(o.path, []byte(b.String()), 0o644)
}

// clip keeps the first n lines of s for a prompt.
func clip(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:n], "\n") + fmt.Sprintf("\n... %d more lines", len(lines)-n)
}

// RebuildSelf rebuilds the running dfinstall from dir after a pull changed
// its source, and returns the binary to re-exec. "" means carry on with the
// current one, which still installs; it just lacks what the pull brought.
func RebuildSelf(ctx context.Context, dir string) string {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		core.AlwaysWarn("dfinstall's source changed, but the running binary could not be located: %v", err)
		return ""
	}
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	var target string
	switch exe {
	case resolve(filepath.Join(dir, "bin", "dfinstall")):
		target = "build-all"
	case resolve(core.HomeTarget(".local", "bin", "dfinstall")):
		target = "install-bin"
	default:
		core.AlwaysWarn("dfinstall's source changed; rebuild it with `make -C %s install-bin`", dir)
		return ""
	}
	core.Status("dfinstall's source changed; rebuilding it")
	if err := runCmd(ctx, "make", "-C", dir, target); err != nil {
		core.AlwaysWarn("rebuild failed, continuing with the current dfinstall: %v", err)
		return ""
	}
	return exe
}
