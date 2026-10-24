package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/sresarehumantoo/dotfiles/internal/core"
)

// GitHub release asset handling, shared by the deb, AppImage and
// release-binary installers.
//
// Each of those carried its own copy: the same anonymous response struct, the
// same arch-token map, and the same fetch-parse-select sequence, three times
// over. Collapsing them also makes asset selection testable from a fixture,
// with no network involved.

// ghAsset is one downloadable file attached to a GitHub release.
type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	// Digest is GitHub's "sha256:<hex>" for the uploaded file. Assets uploaded
	// before mid-2025 have none.
	Digest string `json:"digest"`
}

// downloadVerified fetches a release asset to dst and checks it against the
// sha256 GitHub recorded at upload. An asset with no digest is refused rather
// than trusted: several callers go on to run it as root (dpkg -i).
//
// The digest comes from the same API as the asset, so it does not defend
// against a compromised upstream. What it does pin is that the bytes installed
// are the bytes the release listing described: not a truncated download, an
// error page, or a file swapped between the fetch and the install.
func downloadVerified(ctx context.Context, a ghAsset, dst string) error {
	want, ok := strings.CutPrefix(a.Digest, "sha256:")
	if !ok || want == "" {
		return fmt.Errorf("%s has no published sha256 digest; refusing to install it", a.Name)
	}
	if err := githubDownload(ctx, a.URL, dst); err != nil {
		return fmt.Errorf("download %s: %w", a.Name, err)
	}
	got, err := core.FileHash(dst)
	if err != nil {
		return fmt.Errorf("hash %s: %w", a.Name, err)
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("sha256 mismatch for %s: want %s, got %s", a.Name, want, got)
	}
	return nil
}

// latestAssets returns the assets attached to a repo's latest release.
func latestAssets(ctx context.Context, repo string) ([]ghAsset, error) {
	_, assets, err := latestRelease(ctx, repo)
	return assets, err
}

// latestRelease returns the tag and assets of a repo's latest release.
func latestRelease(ctx context.Context, repo string) (string, []ghAsset, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	out, err := githubProbe(ctx, apiURL)
	if err != nil {
		return "", nil, fmt.Errorf("fetch releases for %s: %w", repo, err)
	}

	var release struct {
		Tag    string    `json:"tag_name"`
		Assets []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal(out, &release); err != nil {
		return "", nil, fmt.Errorf("parse releases JSON for %s: %w", repo, err)
	}
	return release.Tag, release.Assets, nil
}

// GitHub through a mirror.
//
// A corporate Nexus can proxy github.com and api.github.com as raw
// repositories. Measured against Nexus CE 3.96: the release API answers with
// GitHub's own JSON, and an asset download follows GitHub's redirect to its
// object store and matches the published digest, so downloadVerified holds
// either way. When npm points at a Nexus, that Nexus is asked which of its
// proxies front GitHub, and every GitHub fetch tries the mirror first and
// GitHub itself second, the same order opencode's npm source uses.

type githubMirror struct {
	api string // proxy of https://api.github.com/, ending in "/"
	web string // proxy of https://github.com/, ending in "/"
}

var (
	githubMirrorOnce sync.Once
	githubMirrorSeen githubMirror
)

// currentGitHubMirror discovers the mirror once per process.
func currentGitHubMirror(ctx context.Context) githubMirror {
	githubMirrorOnce.Do(func() {
		githubMirrorSeen = findGitHubMirror(ctx, npmRegistry(ctx))
	})
	return githubMirrorSeen
}

// nexusBase returns the root of the Nexus that registry is a repository of
// ("https://host/nexus/" for "https://host/nexus/repository/npm/"), or "" when
// it is not a Nexus repository URL. Credentials are dropped: a URL holding
// them would end up in error messages.
func nexusBase(registry string) string {
	u, err := url.Parse(registry)
	if err != nil || u.Host == "" {
		return ""
	}
	i := strings.Index(u.Path, "/repository/")
	if i < 0 {
		return ""
	}
	u.User = nil
	u.Path = u.Path[:i+1]
	u.RawPath, u.RawQuery, u.Fragment = "", "", ""
	return u.String()
}

// findGitHubMirror asks the Nexus behind a mirrored npm registry for raw
// proxies of GitHub. It relies on anonymous read of the repository list, which
// Nexus allows whenever anonymous access is on; anything else means no mirror.
func findGitHubMirror(ctx context.Context, registry string) githubMirror {
	if !npmMirrored(registry) {
		return githubMirror{}
	}
	base := nexusBase(registry)
	if base == "" {
		return githubMirror{}
	}
	out, err := runNetProbe(ctx, "curl", "-fsSL", "--connect-timeout", "10", "--max-time", "30",
		base+"service/rest/v1/repositories")
	if err != nil {
		core.Debug("no GitHub mirror from %s: %v", base, err)
		return githubMirror{}
	}
	return parseNexusRepos(base, out)
}

// parseNexusRepos picks the GitHub proxies out of a Nexus repository listing.
func parseNexusRepos(base string, listing []byte) githubMirror {
	var repos []struct {
		Name       string `json:"name"`
		Format     string `json:"format"`
		Type       string `json:"type"`
		Attributes struct {
			Proxy struct {
				RemoteURL string `json:"remoteUrl"`
			} `json:"proxy"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(listing, &repos); err != nil {
		return githubMirror{}
	}
	var m githubMirror
	for _, r := range repos {
		if r.Format != "raw" || r.Type != "proxy" || r.Name == "" {
			continue
		}
		repoURL := base + "repository/" + url.PathEscape(r.Name) + "/"
		switch strings.TrimSuffix(r.Attributes.Proxy.RemoteURL, "/") {
		case "https://api.github.com":
			m.api = repoURL
		case "https://github.com":
			m.web = repoURL
		}
	}
	return m
}

// urlsVia returns where to fetch raw from, in order: the mirror's copy first
// when the mirror proxies that host, then raw itself.
func (m githubMirror) urlsVia(raw string) []string {
	for _, p := range []struct{ prefix, mirror string }{
		{"https://api.github.com/", m.api},
		{"https://github.com/", m.web},
	} {
		if p.mirror != "" && strings.HasPrefix(raw, p.prefix) {
			return []string{p.mirror + strings.TrimPrefix(raw, p.prefix), raw}
		}
	}
	return []string{raw}
}

// githubProbe fetches a GitHub URL's body, through the mirror when there is one.
func githubProbe(ctx context.Context, raw string) ([]byte, error) {
	var err error
	for i, u := range currentGitHubMirror(ctx).urlsVia(raw) {
		var out []byte
		if out, err = runNetProbe(ctx, "curl", curlArgs(u)...); err == nil {
			return out, nil
		}
		if i == 0 && u != raw {
			core.Warn("GitHub mirror failed, trying GitHub: %v", err)
		}
	}
	return nil, err
}

// githubDownload saves a GitHub URL to dst, through the mirror when there is one.
func githubDownload(ctx context.Context, raw, dst string) error {
	var err error
	for i, u := range currentGitHubMirror(ctx).urlsVia(raw) {
		if err = runNetCmd(ctx, "curl", curlArgs(u, "-o", dst)...); err == nil {
			return nil
		}
		if i == 0 && u != raw {
			core.Warn("GitHub mirror failed, trying GitHub: %v", err)
		}
	}
	return err
}

var semverRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// sameRelease reports whether a binary's --version output names the release
// tagged tag. Tags and version strings disagree on a leading "v", so both are
// reduced to their first x.y.z.
func sameRelease(versionOutput []byte, tag string) bool {
	have := semverRe.Find(versionOutput)
	want := semverRe.FindString(tag)
	return have != nil && want != "" && string(have) == want
}

// releaseCurrent reports whether the binary at bin already is the release
// tagged tag. A binary that will not run is not current.
func releaseCurrent(ctx context.Context, bin, tag string, versionArgs ...string) bool {
	if len(versionArgs) == 0 {
		versionArgs = []string{"--version"}
	}
	out, err := runProbe(ctx, bin, versionArgs...)
	return err == nil && sameRelease(out, tag)
}

// upgradeable reports whether --upgrade should check path, a release binary
// dfinstall installed earlier, against the latest release.
func upgradeable(path string) bool {
	if !core.UpgradeMode || path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// archTokensFor returns the substrings release assets use to name goarch.
// Order is irrelevant to selection — the search iterates assets on the outside
// and only asks whether any token matches — so the three call sites listing
// these in different orders were always equivalent.
func archTokensFor(goarch string) []string {
	switch goarch {
	case "amd64":
		return []string{"x86_64", "amd64", "x64"}
	case "arm64":
		return []string{"aarch64", "arm64"}
	default:
		return []string{goarch}
	}
}

// sidecarSuffixes are release artifacts that sit alongside the real asset:
// checksums, signatures, SBOMs. Never the thing we want to install.
var sidecarSuffixes = []string{".sha256", ".sig", ".asc", ".sbom", ".pem"}

// assetFilter narrows a release's assets down to the one to download.
type assetFilter struct {
	// ArchTokens are the acceptable architecture substrings. Empty means any.
	ArchTokens []string
	// Suffix, when set, is a required filename suffix (".deb", ".appimage").
	Suffix string
	// Contains, when set, is a required substring — the registry's
	// asset_pattern, e.g. "linux-musl".
	Contains string
	// Exclude drops any asset whose name contains one of these substrings.
	Exclude []string
	// SkipSidecars drops checksums, signatures and SBOMs.
	SkipSidecars bool
	// LinuxOnly drops darwin/windows assets when a release ships several.
	LinuxOnly bool
	// DarwinOnly keeps only assets that name macOS. It can't mirror LinuxOnly
	// by dropping other platforms: Linux builds often name no OS (a .deb, a
	// -musl tarball), while macOS builds always do.
	DarwinOnly bool
}

// pickAsset returns the first asset satisfying the filter.
func pickAsset(assets []ghAsset, f assetFilter) (ghAsset, bool) {
	matches := filterAssets(assets, f)
	if len(matches) == 0 {
		return ghAsset{}, false
	}
	return matches[0], true
}

// filterAssets returns every asset satisfying the filter, in listing order.
func filterAssets(assets []ghAsset, f assetFilter) []ghAsset {
	var out []ghAsset
	for _, a := range assets {
		lower := strings.ToLower(a.Name)

		if f.Suffix != "" && !strings.HasSuffix(lower, strings.ToLower(f.Suffix)) {
			continue
		}
		if f.Contains != "" && !strings.Contains(lower, strings.ToLower(f.Contains)) {
			continue
		}
		if f.SkipSidecars && hasAnySuffix(lower, sidecarSuffixes) {
			continue
		}
		if f.LinuxOnly && isNonLinuxAsset(lower) {
			continue
		}
		if f.DarwinOnly && !containsAny(lower, darwinTokens) {
			continue
		}
		if len(f.ArchTokens) > 0 && !containsAny(lower, f.ArchTokens) {
			continue
		}
		if containsAny(lower, f.Exclude) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// debugPackageMarkers name Debian debug-symbol packages, which releases list
// beside (and often before) the real package and which only make sense
// installed on top of it.
var debugPackageMarkers = []string{"-dbgsym_", "-dbg_"}

// pickDebForDistro chooses among a release's .deb candidates for this arch.
// Repackagers like ghostty-debian publish one build per distro release
// (ghostty_1.3.1-6.trixie_amd64.deb, ...noble_amd64.deb), and another
// release's build either fails its dependencies or links against the wrong
// libraries. codenames are tried in order; a lone candidate is taken as is,
// since a project shipping a single .deb names no distro at all.
func pickDebForDistro(candidates []ghAsset, codenames []string) (ghAsset, error) {
	switch len(candidates) {
	case 0:
		return ghAsset{}, fmt.Errorf("no .deb in the release")
	case 1:
		return candidates[0], nil
	}
	var tried []string
	for _, c := range codenames {
		if c == "" {
			continue
		}
		tried = append(tried, c)
		for _, a := range candidates {
			if slices.Contains(nameTokens(a.Name), c) {
				return a, nil
			}
		}
	}
	names := make([]string, len(candidates))
	for i, a := range candidates {
		names[i] = a.Name
	}
	return ghAsset{}, fmt.Errorf("no .deb for %s among: %s", strings.Join(tried, "/"), strings.Join(names, ", "))
}

// nameTokens splits an asset name on everything but letters and digits, so a
// codename matches as a whole word ("sid" does not match "inside").
func nameTokens(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

func hasAnySuffix(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

var darwinTokens = []string{"darwin", "macos", "apple", "osx"}

func isNonLinuxAsset(lower string) bool {
	return strings.Contains(lower, "darwin") ||
		strings.Contains(lower, "windows") ||
		strings.Contains(lower, ".exe")
}

// currentArchTokens is archTokensFor for the running architecture.
func currentArchTokens() []string { return archTokensFor(runtime.GOARCH) }
