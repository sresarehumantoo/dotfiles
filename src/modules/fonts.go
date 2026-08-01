package modules

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		for _, f := range downloadedFonts {
			core.Info("would download %s (%s) to %s", f.family, nerdFontsTag,
				core.XDGDataTarget("fonts", f.dir))
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

	// Downloaded families — the terminal's primary font.
	for _, f := range downloadedFonts {
		if fontFamilyInstalled(ctx, f.family) {
			core.Ok("font already installed: %s", f.family)
			continue
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
	// context.Background(): Status carries no context to inherit — this probe is
	// the sanctioned exception (see CLAUDE.md subprocess convention).
	var missing []string
	for _, f := range downloadedFonts {
		if !fontFamilyInstalled(context.Background(), f.family) {
			missing = append(missing, f.family)
		}
	}
	if len(missing) > 0 {
		s.Extra = "not downloaded: " + strings.Join(missing, ", ")
	}
	return s
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

	// 1. Download the archive.
	archivePath := filepath.Join(tmp, f.archive)
	archiveURL := fmt.Sprintf(
		"https://github.com/ryanoasis/nerd-fonts/releases/download/%s/%s", nerdFontsTag, f.archive)
	if err := runCmd(ctx, "curl", "-fsSL", archiveURL, "-o", archivePath); err != nil {
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

	// 3. Extract only the wanted faces into a fresh subdir. Rebuild it from
	//    scratch so a prior partial extract can't leave stray weights behind.
	if err := core.RemoveManagedDir(destDir); err != nil {
		return err
	}
	if err := core.EnsureDir(destDir); err != nil {
		return err
	}
	if err := extractFaces(archivePath, destDir, f.faces); err != nil {
		// Don't leave a half-populated dir that Status would count as present.
		_ = core.RemoveManagedDir(destDir)
		return err
	}
	return nil
}

// fetchExpectedSHA returns the expected sha256 for an asset from the release's
// SHA-256.txt (lines are "<hex>  <filename>").
func fetchExpectedSHA(ctx context.Context, asset string) (string, error) {
	url := fmt.Sprintf(
		"https://github.com/ryanoasis/nerd-fonts/releases/download/%s/SHA-256.txt", nerdFontsTag)
	out, err := runNetProbe(ctx, "curl", "-fsSL", url)
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
