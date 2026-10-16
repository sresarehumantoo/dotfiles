package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	before, _ := gitOut(ctx, dir, "rev-parse", "HEAD")
	if err := runNetCmd(ctx, "git", "-C", dir, "pull", "--ff-only", "-q"); err != nil {
		core.Warn("%s not updated: %v", name, err)
		return
	}
	if after, _ := gitOut(ctx, dir, "rev-parse", "HEAD"); after != before {
		core.Changed("updated %s", name)
		return
	}
	core.Ok("%s already up to date", name)
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

	rescueReplacedLinks(ctx, dir, core.DotfilesDir(), managedLinks())
	moveLocalEdits(ctx, dir)

	upstream, err := gitOut(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		core.AlwaysWarn("%s has no upstream branch; not pulling", dir)
		return false, nil
	}
	if core.DryRun {
		core.Notice("would fetch %s (counts below are as of the last fetch)", upstream)
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
	case behind == 0 && core.DryRun:
		core.Status("dotfiles clone was up to date with %s at the last fetch", upstream)
		return false, nil
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
		core.Notice("would fast-forward %d commit(s) from %s", behind, upstream)
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
// Links name sources under root, the canonical clone, which before install
// all adopts dir may be a different checkout; they are mapped into dir.
func rescueReplacedLinks(ctx context.Context, dir, root string, links core.LinkSet) {
	for _, l := range links {
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

// managedLinks is every link the modules in install all would make.
func managedLinks() core.LinkSet {
	var links core.LinkSet
	for _, m := range core.AllModules() {
		if a, ok := m.(appModule); ok && !a.AppInstalled() {
			continue
		}
		if le, ok := m.(core.LinkExporter); ok && !core.SkipInAll(m.Name()) {
			links = append(links, le.Links()...)
		}
	}
	return links
}

// adoptReplacedLink copies dst into the clone. Without a terminal to ask, it
// adopts: that only edits the clone, which git can undo, whereas the default
// of linking over it would quietly drop the setting into a .bak file.
func adoptReplacedLink(ctx context.Context, src, dst string, content []byte) {
	if core.DryRun {
		core.Notice("would offer to copy %s, a file where a link belongs, into %s", dst, src)
		return
	}
	if canPrompt() {
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
	core.Status("copied %s, which had replaced its link, into %s", dst, src)
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
	// overrideGit sets changed keys with git config, in a file included at
	// the end of gitconfig, so its values win.
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
		o = override{core.HomeTarget(".gitconfig.user"), overrideGit}
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

// gitChange is one key whose values differ between two versions of a git
// config file.
type gitChange struct {
	key           string
	before, after []string
}

// gitConfigChanges compares two parsed configs key by key, in the order keys
// first appear, so reformatting or moving a key between identical sections
// is no change.
func gitConfigChanges(before, after [][2]string) []gitChange {
	vals := func(entries [][2]string) (map[string][]string, []string) {
		m := map[string][]string{}
		var order []string
		for _, kv := range entries {
			if _, seen := m[kv[0]]; !seen {
				order = append(order, kv[0])
			}
			m[kv[0]] = append(m[kv[0]], kv[1])
		}
		return m, order
	}
	b, bOrder := vals(before)
	a, aOrder := vals(after)
	var changes []gitChange
	for _, k := range aOrder {
		if !slices.Equal(b[k], a[k]) {
			changes = append(changes, gitChange{k, b[k], a[k]})
		}
	}
	for _, k := range bOrder {
		if _, ok := a[k]; !ok {
			changes = append(changes, gitChange{k, b[k], nil})
		}
	}
	return changes
}

// gitMultiValued reports keys where git uses every value, not the last, so a
// later file adds to the list instead of replacing it. Names are as git
// config --list prints them: section and variable lowercased.
func gitMultiValued(key string) bool {
	section, rest, _ := strings.Cut(key, ".")
	name := rest[strings.LastIndex(rest, ".")+1:]
	switch {
	case section == "credential" && name == "helper",
		section == "url" && (name == "insteadof" || name == "pushinsteadof"),
		section == "remote" && (name == "fetch" || name == "push" || name == "url" || name == "pushurl"),
		(section == "include" || section == "includeif") && name == "path",
		key == "http.extraheader", key == "safe.directory":
		return true
	}
	return false
}

// planGitMove works out the git config calls that reproduce an edit from a
// file loaded after gitconfig. A changed single value is set there and wins.
// A list can only grow that way, and nothing can be unset, so a removed key
// or a rewritten list cannot move.
func planGitMove(ctx context.Context, dir, rel string) (movedEdit, bool) {
	before, after, ok := gitConfigVersions(ctx, dir, rel)
	if !ok {
		return movedEdit{}, false
	}
	var entries [][2]string
	for _, c := range gitConfigChanges(before, after) {
		switch {
		case len(c.after) == 0:
			return movedEdit{}, false
		case gitMultiValued(c.key) || len(c.before) > 1 || len(c.after) > 1:
			if len(c.after) <= len(c.before) || !slices.Equal(c.after[:len(c.before)], c.before) {
				return movedEdit{}, false
			}
			for _, v := range c.after[len(c.before):] {
				entries = append(entries, [2]string{c.key, v})
			}
		default:
			entries = append(entries, [2]string{c.key, c.after[0]})
		}
	}
	return movedEdit{entries: entries}, true
}

// gitConfigVersions parses rel as committed and as it is in the worktree.
func gitConfigVersions(ctx context.Context, dir, rel string) (before, after [][2]string, ok bool) {
	head, err := runProbe(ctx, "git", "-C", dir, "show", "HEAD:"+rel)
	if err != nil {
		return nil, nil, false
	}
	tmp, err := os.CreateTemp("", "dfinstall-gitconfig-*")
	if err != nil {
		return nil, nil, false
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(head); err != nil || tmp.Close() != nil {
		return nil, nil, false
	}
	before, err1 := gitConfigEntries(ctx, tmp.Name())
	after, err2 := gitConfigEntries(ctx, filepath.Join(dir, rel))
	return before, after, err1 == nil && err2 == nil
}

// withoutPresent drops what the override file already holds, so a move
// repeated on another run, or a setting the user already put there, is not
// written twice.
func withoutPresent(ctx context.Context, o override, edit movedEdit) movedEdit {
	if o.kind == overrideGit {
		have, err := gitConfigEntries(ctx, o.path)
		if err != nil {
			return edit // no file yet: nothing to skip
		}
		var keep [][2]string
		for _, kv := range edit.entries {
			var vals []string
			for _, h := range have {
				if h[0] == kv[0] {
					vals = append(vals, h[1])
				}
			}
			present := slices.Contains(vals, kv[1])
			if !gitMultiValued(kv[0]) && len(vals) > 0 {
				present = vals[len(vals)-1] == kv[1] // the value that wins
			}
			if !present {
				keep = append(keep, kv)
			}
		}
		return movedEdit{entries: keep}
	}
	data, err := os.ReadFile(o.path)
	if err != nil {
		return edit
	}
	have := map[string]bool{}
	for _, l := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(l); t != "" {
			have[t] = true
		}
	}
	var keep []string
	for _, l := range edit.lines {
		if t := strings.TrimSpace(l); t == "" || !have[t] {
			keep = append(keep, l)
		}
	}
	return movedEdit{lines: trimBlank(keep)}
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
			core.Notice("%s has local edits with no override file to move them to; kept in the clone. Commit them to keep them everywhere, or revert: git -C %s checkout -- %s", rel, dir, rel)
			continue
		}
		edit, ok := planMove(ctx, dir, rel, o.kind)
		if !ok {
			core.Notice("%s has local edits that change or remove upstream lines, which %s cannot undo; kept in the clone. Commit them to keep them everywhere, or revert: git -C %s checkout -- %s", rel, o.path, dir, rel)
			continue
		}
		if edit.empty() {
			continue
		}
		edit = withoutPresent(ctx, o, edit)
		already := edit.empty()
		title := fmt.Sprintf("Move this machine's edits to %s into %s?", rel, o.path)
		desc := clip(edit.String(), 30) + "\n\nThey keep applying from there, and the clone goes back to upstream's version."
		yes, done := "Move them", fmt.Sprintf("moved edits to %s into %s", rel, o.path)
		if already {
			title = fmt.Sprintf("%s already holds this machine's edits to %s. Restore the clone's copy?", o.path, rel)
			desc = "Nothing is written; the clone goes back to upstream's version and the settings keep applying from there."
			yes, done = "Restore it", fmt.Sprintf("restored %s; %s already held its edits", rel, o.path)
		}
		switch {
		case core.DryRun:
			core.Notice("would offer to move the edits to %s into %s:\n%s", rel, o.path, edit)
			continue
		case !canPrompt():
			core.Notice("%s has local edits that could move to %s; rerun --upgrade in a terminal to move them", rel, o.path)
			continue
		}
		move := true
		form := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(desc).
				Affirmative(yes).
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
		core.Status("%s", done)
	}
}

func applyMove(ctx context.Context, o override, rel string, edit movedEdit) error {
	if err := core.CheckTarget(o.path); err != nil {
		return err
	}
	if o.kind == overrideGit {
		for _, kv := range edit.entries {
			mode := "--replace-all" // a single value: this one wins
			if gitMultiValued(kv[0]) {
				mode = "--add"
			}
			if err := runCmd(ctx, "git", "config", "--file", o.path, mode, kv[0], kv[1]); err != nil {
				return err
			}
		}
		return nil
	}
	if edit.empty() {
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

// UpgradeClone converges dir, then rebuilds dfinstall when its source there
// differs from what this binary was built from, which also covers a clone
// pulled by hand since. Returns the rebuilt binary, or "" when there is none
// to switch to. Failures are reported and the install goes on with the clone
// as it is.
func UpgradeClone(ctx context.Context, dir string) string {
	changed, err := ConvergeClone(ctx, dir)
	if err != nil {
		core.AlwaysWarn("could not update %s: %v", dir, err)
	}
	stale := changed || BinaryStale(ctx, dir, core.Version)
	if stale && core.DryRun {
		core.Notice("would rebuild dfinstall from %s", dir)
	}
	// Before any re-exec, which would drop what quiet mode buffered.
	core.FlushWarnings()
	if !stale || core.DryRun {
		return ""
	}
	return rebuildSelf(ctx, dir)
}

// rebuildSelf rebuilds the running binary from dir and returns its path. ""
// means carry on with the current one, which still installs; it just lacks
// what the pull brought.
func rebuildSelf(ctx context.Context, dir string) string {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		core.AlwaysWarn("dfinstall is out of date, but the running binary could not be located: %v", err)
		return ""
	}
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	// The CLI or the MCP server; both targets build both binaries.
	name := filepath.Base(exe)
	var target string
	switch exe {
	case resolve(filepath.Join(dir, "bin", name)):
		target = "build-all"
	case resolve(core.HomeTarget(".local", "bin", name)):
		target = "install-bin"
	default:
		core.AlwaysWarn("dfinstall is out of date with %s; rebuild it with `make -C %s install-bin`", dir, dir)
		return ""
	}
	core.Status("rebuilding dfinstall from %s", dir)
	if err := runCmd(ctx, "make", "-C", dir, target); err != nil {
		core.AlwaysWarn("rebuild failed, continuing with the current dfinstall: %v", err)
		return ""
	}
	return exe
}

// describedCommit matches the commit `git describe` appends to a version
// built between tags: v1.4.0-3-gabc1234, optionally -dirty.
var describedCommit = regexp.MustCompile(`-g([0-9a-f]{7,40})(-dirty)?$`)

// BinaryStale reports whether a dfinstall stamped with version was built from
// source other than what dir's HEAD holds. A version that names no commit in
// dir, such as "dev" from a plain go build, counts as stale. A binary built
// from uncommitted changes is compared by the commit it was built on.
func BinaryStale(ctx context.Context, dir, version string) bool {
	if _, err := gitOut(ctx, dir, "rev-parse", "--verify", "HEAD"); err != nil {
		return false // not a checkout: nothing to rebuild from
	}
	var rev string
	if m := describedCommit.FindStringSubmatch(version); m != nil {
		rev = m[1]
	} else {
		rev = strings.TrimSuffix(version, "-dirty") // built exactly on a tag
	}
	commit, err := gitOut(ctx, dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil || commit == "" {
		return true
	}
	_, err = runProbe(ctx, "git", "-C", dir, "diff", "--quiet", commit, "HEAD", "--", "src", "go.mod", "go.sum", "Makefile")
	return err != nil
}
