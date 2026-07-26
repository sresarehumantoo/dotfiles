package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
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
}

// latestAssets returns the assets attached to a repo's latest release.
func latestAssets(ctx context.Context, repo string) ([]ghAsset, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	out, err := runNetProbe(ctx, "curl", "-fsSL", apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch releases for %s: %w", repo, err)
	}

	var release struct {
		Assets []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal(out, &release); err != nil {
		return nil, fmt.Errorf("parse releases JSON for %s: %w", repo, err)
	}
	return release.Assets, nil
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
	// SkipSidecars drops checksums, signatures and SBOMs.
	SkipSidecars bool
	// LinuxOnly drops darwin/windows assets when a release ships several.
	LinuxOnly bool
}

// pickAsset returns the first asset satisfying the filter.
func pickAsset(assets []ghAsset, f assetFilter) (ghAsset, bool) {
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
		if len(f.ArchTokens) > 0 && !containsAny(lower, f.ArchTokens) {
			continue
		}
		return a, true
	}
	return ghAsset{}, false
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

func isNonLinuxAsset(lower string) bool {
	return strings.Contains(lower, "darwin") ||
		strings.Contains(lower, "windows") ||
		strings.Contains(lower, ".exe")
}

// currentArchTokens is archTokensFor for the running architecture.
func currentArchTokens() []string { return archTokensFor(runtime.GOARCH) }
