package modules

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/ulikunitz/xz"
)

type FontsModule struct{}

func (FontsModule) Name() string { return "fonts" }

// nerdFontsTag pins the Nerd Fonts release the downloaded families come from.
// Bumping it is a deliberate change: update the tag, then re-derive each
// archive's sha from SHA-256.txt on the new tag (the module verifies against it
// at install time, so a stale hash fails loudly rather than installing quietly).
const nerdFontsTag = "v3.4.0"

// downloadedFont is a family fetched from the Nerd Fonts release rather than
// vendored — IosevkaTerm at ~28 MB per version is too large to carry in git
// (see .git history rationale in the module notes). Everything about where its
// faces live is decided here, in one place, so Install, Status and Uninstall
// can't disagree about what to remove — the same discipline as
// toolkit_artifact.go, where a second copy of the mapping means uninstall
// deletes the wrong path.
type downloadedFont struct {
	family  string   // fontconfig family name, used only for detection (fc-list -q)
	archive string   // asset filename in the release, e.g. "IosevkaTerm.tar.xz"
	dir     string   // subdir under $XDG_DATA_HOME/fonts, e.g. "IosevkaTerm"
	faces   []string // exact basenames to extract — nothing else is written
}

// iosevkaTerm is the terminal's primary font. The archive ships every weight in
// three variants (Mono, Propo, and the default proportional-metrics build);
// only the four canonical faces of the *default* build are wanted. Mono clamps
// icons to one cell (tiny logos); Propo has 853 distinct advance widths and is
// not monospace — neither may ever be installed for a terminal.
var iosevkaTerm = downloadedFont{
	family:  "IosevkaTerm Nerd Font",
	archive: "IosevkaTerm.tar.xz",
	dir:     "IosevkaTerm",
	faces: []string{
		"IosevkaTermNerdFont-Regular.ttf",
		"IosevkaTermNerdFont-Bold.ttf",
		"IosevkaTermNerdFont-Italic.ttf",
		"IosevkaTermNerdFont-BoldItalic.ttf",
	},
}

// downloadedFonts is the full set of families fetched on install.
var downloadedFonts = []downloadedFont{iosevkaTerm}

// tagStamp records which nerdFontsTag produced the faces sitting beside it. It
// is what makes a tag bump actionable: without it the only thing observable
// about an installed family is its name, which is version-independent, so
// changing nerdFontsTag would silently no-op on every machine that already had
// the font (the documented "bump the tag" workflow did exactly nothing).
const tagStamp = ".nerd-fonts-tag"

// fontInstalled reports whether *this module's own copy* of a family is present,
// and which tag it came from.
//
// Deliberately a filesystem check on the directory the module owns, not a
// fontconfig query. `fc-list` answers "does some font by this name exist
// anywhere", which is a different question and answers it wrongly for our
// purposes three ways: a hand-installed copy elsewhere suppressed the download
// forever, an outdated copy looked current, and Uninstall reported success while
// leaving the family resolvable. It also made every machine without fontconfig
// re-download ~28 MB on each run, since a missing fc-list reads as "absent".
func fontInstalled(f downloadedFont) (present bool, tag string) {
	dir := core.XDGDataTarget("fonts", f.dir)
	if dir == "" {
		return false, ""
	}
	for _, face := range f.faces {
		st, err := os.Stat(filepath.Join(dir, face))
		if err != nil || st.Size() == 0 {
			return false, ""
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, tagStamp))
	if err != nil {
		// Faces present but unstamped: installed by a build that predates the
		// stamp. Treat as an unknown version so it gets refreshed.
		return true, ""
	}
	return true, strings.TrimSpace(string(b))
}

// tagOrUnknown renders an empty tag readably.
func tagOrUnknown(tag string) string {
	if tag == "" {
		return "unknown version"
	}
	return tag
}

// Links is the single source of truth for the *vendored* fonts — the offline
// floor. fontconfig follows symlinked font files, so these need no copy.
// MesloLGS NF Regular supplies every glyph the configs print (✓ ✗ ✘ ⚠ ⚙),
// powerline separators and the Nerd sentinels from one face, so a box that
// never reaches the network still renders a working p10k prompt.
func (FontsModule) Links() core.LinkSet {
	return core.LinkSet{
		{
			Src: core.ConfigPath("fonts", "MesloLGS NF Regular.ttf"),
			Dst: core.XDGDataTarget("fonts", "MesloLGS NF Regular.ttf"),
		},
	}
}

func (m FontsModule) Install(ctx context.Context) error {
	if core.DryRun {
		core.Info("would link vendored fonts (MesloLGS offline floor)")
		for _, name := range m.legacyArtifacts() {
			core.Info("would remove legacy font artifact: %s", name)
		}
		for _, f := range downloadedFonts {
			present, tag := fontInstalled(f)
			switch {
			case present && tag == nerdFontsTag:
				core.Info("%s (%s) already installed — would leave it alone", f.family, nerdFontsTag)
			case present:
				core.Info("would update %s from %s to %s", f.family, tagOrUnknown(tag), nerdFontsTag)
			default:
				core.Info("would download %s (%s) to %s", f.family, nerdFontsTag,
					core.XDGDataTarget("fonts", f.dir))
			}
		}
		return nil
	}

	core.Info("Installing fonts...")

	// Vendored offline floor (symlinked). A newly-created link means fontconfig
	// hasn't seen the file yet, so the cache needs refreshing below.
	changed := m.Links().Status("fonts").Missing > 0
	if err := m.Links().Apply(); err != nil {
		return err
	}

	// Retire what the pre-IosevkaTerm module left behind. Must run after Apply,
	// which is what creates the displaced .bak this cleans up.
	if m.cleanLegacyArtifacts() {
		changed = true
	}

	// Downloaded families — the terminal's primary font.
	for _, f := range downloadedFonts {
		present, tag := fontInstalled(f)
		switch {
		case present && tag == nerdFontsTag:
			core.Ok("font already installed: %s (%s)", f.family, nerdFontsTag)
			continue
		case present:
			core.Info("updating %s: %s -> %s", f.family, tagOrUnknown(tag), nerdFontsTag)
		case fontFamilyInstalled(ctx, f.family):
			// Present to fontconfig but absent from the directory this module
			// owns — a hand-installed copy. Install a managed one anyway, or the
			// pin is unenforceable and Uninstall can't work; say so, because the
			// unmanaged copy stays behind and is the user's to remove.
			core.Warn("%s is installed outside %s and unmanaged — installing a "+
				"managed copy; remove the other to avoid duplicates",
				f.family, core.XDGDataTarget("fonts", f.dir))
		}
		if err := installDownloadedFont(ctx, f); err != nil {
			core.Warn("could not install %s: %v — MesloLGS floor remains in place", f.family, err)
			continue
		}
		core.Ok("installed font: %s (%s)", f.family, nerdFontsTag)
		changed = true
	}

	if changed {
		refreshFontCache(ctx)
		core.Status("Fonts changed — fully restart your terminal (a config reload is not enough) to load them.")
	}

	core.Ok("Fonts done")
	return nil
}

func (m FontsModule) Uninstall(ctx context.Context) error {
	// Vendored (symlinked) floor.
	if err := m.Links().Remove(); err != nil {
		return err
	}
	// Downloaded families each own a subdir, so removal is bounded.
	for _, f := range downloadedFonts {
		dir := core.XDGDataTarget("fonts", f.dir)
		if dir == "" {
			continue // $HOME unresolved; nothing we created to remove
		}
		if err := core.RemoveManagedDir(dir); err != nil {
			core.Warn("could not remove %s: %v", dir, err)
		}
	}
	refreshFontCache(ctx)
	core.Ok("Fonts uninstalled")
	return nil
}

func (m FontsModule) Status() core.ModuleStatus {
	// Linked/Missing count the vendored links only, so the count stays exactly
	// what Links() exports (tests/linkset_test.go asserts this). Downloaded
	// families aren't links — they're reported in the INFO column instead.
	s := m.Links().Status("fonts")
	notes := fontNotes()
	if legacy := m.legacyArtifacts(); len(legacy) > 0 {
		notes = append(notes, fmt.Sprintf("%d legacy artifact(s) to clean", len(legacy)))
	}
	s.Extra = strings.Join(notes, "; ")
	return s
}

// fontNotes describes anything wrong with the downloaded families, as short
// phrases. Shared by Status and the doctor check so the two can't disagree —
// no subprocess, so it is safe from Status(), which carries no context.
func fontNotes() []string {
	var notes []string
	for _, f := range downloadedFonts {
		present, tag := fontInstalled(f)
		switch {
		case !present:
			notes = append(notes, "not downloaded: "+f.family)
		case tag != nerdFontsTag:
			notes = append(notes, fmt.Sprintf("%s is %s, want %s",
				f.family, tagOrUnknown(tag), nerdFontsTag))
		}
		// Independent of the above: a copy outside the managed directory is a
		// duplicate face whether or not the managed one is present and current.
		if extra := unmanagedFontCopies(f); len(extra) > 0 {
			notes = append(notes, fmt.Sprintf("%d unmanaged file(s) for %s outside %s",
				len(extra), f.family, core.XDGDataTarget("fonts", f.dir)))
		}
	}
	return notes
}

// unmanagedFontCopies lists files fontconfig resolves for a family that sit
// outside the directory this module owns.
//
// ⚠ THE INSTALL-TIME WARNING WAS NOT ENOUGH, AND THE GAP WENT GREEN. Install
// says "installed outside ... and unmanaged" exactly once, in a branch reached
// only while the managed copy is absent. After it installs one, nothing looks
// again — so `status` and `doctor` both reported a clean fonts row on a machine
// where fontconfig was resolving EIGHT copies of the family (four managed, four
// hand-installed, identical bytes). A report that goes green while the
// condition persists is the same failure the fonts doctor check was rewritten
// to fix; this closes it at the one place both surfaces read from.
//
// ⚠ NOT `fc-list -q <family>`, which is what install uses. That answers "does
// this family exist anywhere", and once a managed copy exists the answer is
// always yes — it cannot see a duplicate. The question here is about PATHS, so
// it has to list them and filter.
//
// A missing fc-list returns nil rather than a guess: silence beats a false
// report, and reading "absent" from a missing tool is the exact bug that made
// the old gate re-download 28 MB on every run.
//
// ⚠ Scoping comes from the fc-list ARGUMENT, and it was measured rather than
// assumed: `fc-list "IosevkaTerm Nerd Font"` matches that family EXACTLY and
// does not pull in "IosevkaTerm Nerd Font Mono", which is a separate family. If
// it over-matched, every box keeping the Mono build beside this one would be
// nagged forever about a duplicate that is not one.
func unmanagedFontCopies(f downloadedFont) []string {
	managed := core.XDGDataTarget("fonts", f.dir)
	if managed == "" {
		return nil
	}
	if _, err := exec.LookPath("fc-list"); err != nil {
		return nil
	}
	// context.Background: Status() is a fast synchronous read with no context
	// to inherit, the same shared-probe case as resolveWinHome in the wsl
	// module. runProbe still carries ProbeTimeout.
	out, err := runProbe(context.Background(), "fc-list", f.family, "file")
	if err != nil {
		return nil
	}
	return unmanagedPaths(string(out), managed)
}

// unmanagedPaths filters `fc-list <family> file` output down to the paths
// outside managedDir. Split out so the filtering is testable without
// fontconfig, and because the prefix test below has a classic bug in it.
func unmanagedPaths(fcListOutput, managedDir string) []string {
	var extra []string
	for _, line := range strings.Split(fcListOutput, "\n") {
		// fc-list prints "<path>: " for the `file` property.
		path := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ":"))
		if path == "" {
			continue
		}
		// ⚠ The separator is load-bearing. Without it a sibling directory whose
		// name merely starts with the managed one — IosevkaTermPropo beside
		// IosevkaTerm — reads as managed and its faces are never reported.
		if strings.HasPrefix(path, managedDir+string(os.PathSeparator)) {
			continue
		}
		extra = append(extra, path)
	}
	sort.Strings(extra)
	return extra
}

// legacyGlob matches every face the pre-IosevkaTerm module could have put in the
// fonts directory: it copied HackNerdFont-Regular.ttf, and on its download path
// ran `unzip -qo Hack.zip -d <fontdir>`, which dumped the whole archive flat —
// including the Mono and Propo builds this module explicitly refuses to install.
const legacyGlob = "HackNerdFont*.ttf"

// legacyArtifacts lists files in the fonts directory that the old module left
// behind and this one should retire. Deliberately narrow: only faces matching
// legacyGlob, plus a .bak that LinkFile displaced and that is byte-identical to
// the vendored font which replaced it. Generic leftovers from the unzip
// (README.md, LICENSE.md) are NOT listed — those names are too common to claim.
//
// The .bak matters because a font directory has no inert file extensions:
// fontconfig identifies files by content, so "MesloLGS NF Regular.ttf.bak" is
// still a live, registered copy of MesloLGS NF. Left alone, every upgraded
// machine carries a permanent duplicate of the family.
func (m FontsModule) legacyArtifacts() []string {
	dir := core.XDGDataTarget("fonts")
	if dir == "" {
		return nil
	}
	var found []string
	matches, err := filepath.Glob(filepath.Join(dir, legacyGlob))
	if err == nil {
		found = append(found, matches...)
	}
	for _, l := range m.Links() {
		if l.Dst == "" {
			continue
		}
		bak := l.Dst + ".bak"
		if _, err := os.Lstat(bak); err != nil {
			continue
		}
		// Only ours to delete if it is a duplicate of what displaced it.
		if core.FilesMatch(l.Src, bak) {
			found = append(found, bak)
		}
	}
	return found
}

// cleanLegacyArtifacts removes them, reporting whether the font set changed (and
// so whether the fontconfig cache needs rebuilding).
func (m FontsModule) cleanLegacyArtifacts() bool {
	var changed bool
	for _, p := range m.legacyArtifacts() {
		if err := core.CheckTarget(p); err != nil {
			continue
		}
		if err := os.Remove(p); err != nil {
			core.Warn("could not remove legacy font artifact %s: %v", p, err)
			continue
		}
		core.Ok("removed legacy font artifact: %s", filepath.Base(p))
		changed = true
	}
	// A .bak that differs from the vendored font is the user's; say so rather
	// than deleting it, because fontconfig will keep loading it.
	for _, l := range m.Links() {
		if l.Dst == "" {
			continue
		}
		bak := l.Dst + ".bak"
		if _, err := os.Lstat(bak); err == nil && !core.FilesMatch(l.Src, bak) {
			core.Warn("%s differs from the vendored font and was left in place — "+
				"fontconfig still loads it as a duplicate; remove it yourself if unwanted", bak)
		}
	}
	return changed
}

// installDownloadedFont downloads, verifies and extracts one family into its own
// subdirectory. It never touches the rest of the fonts dir.
func installDownloadedFont(ctx context.Context, f downloadedFont) error {
	destDir := core.XDGDataTarget("fonts", f.dir)
	// destDir is assembled from $XDG_DATA_HOME/$HOME and then written to with
	// os.* calls that bypass LinkFile's guard, so check it here.
	if err := core.CheckTarget(destDir); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp("", "dfinstall-fonts-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// 1. Download the archive. runNetCmd, not runCmd: this is a transfer, so it
	//    gets the 10-minute network deadline rather than the 45-minute install
	//    one — a stalled mirror should not hold the run for three quarters of an
	//    hour behind a spinner.
	archivePath := filepath.Join(tmp, f.archive)
	archiveURL := fmt.Sprintf(
		"https://github.com/ryanoasis/nerd-fonts/releases/download/%s/%s", nerdFontsTag, f.archive)
	if err := runNetCmd(ctx, "curl", curlArgs(archiveURL, "-o", archivePath)...); err != nil {
		return fmt.Errorf("download %s: %w", f.archive, err)
	}

	// 2. Verify sha256 against the release's SHA-256.txt before extracting a byte.
	want, err := fetchExpectedSHA(ctx, f.archive)
	if err != nil {
		return err
	}
	got, err := core.FileHash(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: want %s, got %s", f.archive, want, got)
	}

	// 3. Extract into a staging directory *beside* the destination, then swap.
	//    Two reasons it is a sibling rather than under /tmp: the rename stays on
	//    one filesystem, and a working install is never destroyed by a failed
	//    extract. Rebuilding destDir in place meant a corrupt-but-sha-valid
	//    archive deleted good fonts before discovering it could not replace them.
	stage := destDir + ".new"
	if err := core.RemoveManagedDir(stage); err != nil {
		return err
	}
	if err := core.EnsureDir(stage); err != nil {
		return err
	}
	defer func() { _ = core.RemoveManagedDir(stage) }()

	if err := extractFaces(archivePath, stage, f.faces); err != nil {
		return err
	}
	// Stamp the tag alongside the faces, so a later nerdFontsTag bump is
	// detectable. Written before the swap so destDir is never stamp-less.
	stampPath := filepath.Join(stage, tagStamp)
	if err := os.WriteFile(stampPath, []byte(nerdFontsTag+"\n"), 0644); err != nil {
		return err
	}

	// 4. Swap: only now is the old copy removed.
	if err := core.RemoveManagedDir(destDir); err != nil {
		return err
	}
	return os.Rename(stage, destDir)
}

// curlArgs builds the flags every fetch here uses, followed by url and extra.
//
//   - -f    an HTTP error is a non-zero exit, not a saved error page. Without
//     it a 404 body would be sha-checked and reported as a mismatch.
//   - -sS   quiet progress, but keep the failure reason on stderr — which is
//     what probeErr/runCmd surface. -s alone would silence it.
//   - -L    the release asset is a redirect to a CDN.
//   - retries and a connect timeout so a flaky link recovers and a blackholed
//     host fails in seconds rather than sitting there.
//
// Deliberately no --max-time: curl applies it per transfer, so it would fight
// the retries, and the context deadline (NetworkTimeout) is already the real
// ceiling for the whole operation.
func curlArgs(url string, extra ...string) []string {
	args := []string{
		"-fsSL",
		"--connect-timeout", "15",
		"--retry", "3",
		"--retry-delay", "2",
		"--retry-connrefused",
		url,
	}
	return append(args, extra...)
}

// fetchExpectedSHA returns the expected sha256 for an asset from the release's
// SHA-256.txt (lines are "<hex>  <filename>").
func fetchExpectedSHA(ctx context.Context, asset string) (string, error) {
	url := fmt.Sprintf(
		"https://github.com/ryanoasis/nerd-fonts/releases/download/%s/SHA-256.txt", nerdFontsTag)
	out, err := runNetProbe(ctx, "curl", curlArgs(url)...)
	if err != nil {
		return "", fmt.Errorf("fetch SHA-256.txt: %w", err)
	}
	sum, ok := parseSHALine(string(out), asset)
	if !ok {
		return "", fmt.Errorf("no sha256 entry for %s in SHA-256.txt", asset)
	}
	return sum, nil
}

// parseSHALine finds the sha256 for asset in the "<hex>  <filename>" listing.
// Split out so it can be tested without the network.
func parseSHALine(listing, asset string) (string, bool) {
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], true
		}
	}
	return "", false
}

// extractFaces writes exactly the named faces from a .tar.xz into destDir,
// matching on basename so archive layout (flat or nested) doesn't matter. It is
// an error if any wanted face is absent — a silently-incomplete family would
// leave Ghostty synthesizing the missing styles.
func extractFaces(archivePath, destDir string, faces []string) error {
	wanted := wantedSet(faces)

	fh, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer fh.Close()

	xzr, err := xz.NewReader(fh)
	if err != nil {
		return fmt.Errorf("open xz: %w", err)
	}
	tr := tar.NewReader(xzr)

	found := make(map[string]bool, len(faces))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if !wanted[base] {
			continue
		}
		dst := filepath.Join(destDir, base)
		if err := core.CheckTarget(dst); err != nil {
			return err
		}
		if err := writeFile(dst, tr); err != nil {
			return err
		}
		found[base] = true
	}

	if missing := missingFaces(faces, found); len(missing) > 0 {
		return fmt.Errorf("archive missing expected faces: %s", strings.Join(missing, ", "))
	}
	return nil
}

func wantedSet(faces []string) map[string]bool {
	set := make(map[string]bool, len(faces))
	for _, f := range faces {
		set[f] = true
	}
	return set
}

func missingFaces(faces []string, found map[string]bool) []string {
	var missing []string
	for _, f := range faces {
		if !found[f] {
			missing = append(missing, f)
		}
	}
	return missing
}

// writeFile streams r into dst with 0644, truncating any prior content.
func writeFile(dst string, r io.Reader) error {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fontFamilyInstalled reports whether fontconfig knows a family. It uses
// `fc-list -q` (exit 0 iff present) and never fc-match, which always succeeds
// via fallback. A missing fc-list means we can't confirm, so report absent.
func fontFamilyInstalled(ctx context.Context, family string) bool {
	if _, err := exec.LookPath("fc-list"); err != nil {
		return false
	}
	_, err := runProbe(ctx, "fc-list", "-q", family)
	return err == nil
}

// refreshFontCache rebuilds the fontconfig cache for the user's font dir.
// fc-cache's exit code is unreliable (it returns 0 for a missing directory),
// so it is deliberately ignored.
func refreshFontCache(ctx context.Context) {
	if _, err := exec.LookPath("fc-cache"); err != nil {
		return
	}
	dir := core.XDGDataTarget("fonts")
	if dir == "" {
		return
	}
	_ = runCmd(ctx, "fc-cache", "-f", dir)
}
