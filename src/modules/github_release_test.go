package modules

// In-package: pickAsset and assetFilter are unexported. This selection logic
// previously lived inline in three installers, each reachable only by actually
// hitting the GitHub API, so none of it was covered.

import (
	"encoding/json"
	"testing"
)

// A realistic release listing: several architectures, an installer for another
// OS, and the checksum/signature sidecars real projects publish.
const releaseFixture = `{"assets":[
  {"name":"tool-v1.2.3-checksums.txt.sha256","browser_download_url":"https://x/sha"},
  {"name":"tool-v1.2.3-darwin-arm64.tar.gz","browser_download_url":"https://x/darwin"},
  {"name":"tool-v1.2.3-windows-x86_64.exe","browser_download_url":"https://x/win"},
  {"name":"tool-v1.2.3-linux-x86_64.tar.gz","browser_download_url":"https://x/linux-amd64"},
  {"name":"tool-v1.2.3-linux-aarch64.tar.gz","browser_download_url":"https://x/linux-arm64"},
  {"name":"tool-v1.2.3-linux-x86_64-musl.tar.gz","browser_download_url":"https://x/linux-musl"},
  {"name":"tool_1.2.3_amd64.deb","browser_download_url":"https://x/deb-amd64"},
  {"name":"tool_1.2.3_arm64.deb","browser_download_url":"https://x/deb-arm64"},
  {"name":"Tool-1.2.3.AppImage","browser_download_url":"https://x/appimage-noarch"},
  {"name":"Tool-1.2.3-x86_64.AppImage","browser_download_url":"https://x/appimage-amd64"},
  {"name":"tool-v1.2.3-linux-x86_64.tar.gz.sig","browser_download_url":"https://x/sig"}
]}`

func fixtureAssets(t *testing.T) []ghAsset {
	t.Helper()
	var r struct {
		Assets []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal([]byte(releaseFixture), &r); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return r.Assets
}

func TestPickAsset(t *testing.T) {
	assets := fixtureAssets(t)

	cases := []struct {
		name    string
		filter  assetFilter
		wantURL string // "" means expect no match
	}{{
		name:    "deb for amd64",
		filter:  assetFilter{ArchTokens: archTokensFor("amd64"), Suffix: ".deb"},
		wantURL: "https://x/deb-amd64",
	}, {
		name:    "deb for arm64",
		filter:  assetFilter{ArchTokens: archTokensFor("arm64"), Suffix: ".deb"},
		wantURL: "https://x/deb-arm64",
	}, {
		name:    "AppImage matches case-insensitively and needs the arch token",
		filter:  assetFilter{ArchTokens: archTokensFor("amd64"), Suffix: ".AppImage"},
		wantURL: "https://x/appimage-amd64",
	}, {
		// The release binary path takes the first arch-matching asset that
		// isn't a sidecar or another OS — the darwin, windows, .sha256 and
		// .sig entries all precede or surround the one we want.
		name: "release binary skips sidecars and other platforms",
		filter: assetFilter{
			ArchTokens: archTokensFor("amd64"), SkipSidecars: true, LinuxOnly: true,
		},
		wantURL: "https://x/linux-amd64",
	}, {
		name: "asset_pattern narrows to a variant",
		filter: assetFilter{
			ArchTokens: archTokensFor("amd64"), Contains: "musl",
			SkipSidecars: true, LinuxOnly: true,
		},
		wantURL: "https://x/linux-musl",
	}, {
		name: "arm64 release binary",
		filter: assetFilter{
			ArchTokens: archTokensFor("arm64"), SkipSidecars: true, LinuxOnly: true,
		},
		wantURL: "https://x/linux-arm64",
	}, {
		name:    "no deb for an unknown arch",
		filter:  assetFilter{ArchTokens: archTokensFor("riscv64"), Suffix: ".deb"},
		wantURL: "",
	}, {
		name: "pattern that matches nothing",
		filter: assetFilter{
			ArchTokens: archTokensFor("amd64"), Contains: "nonesuch", SkipSidecars: true,
		},
		wantURL: "",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := pickAsset(assets, tc.filter)
			if tc.wantURL == "" {
				if ok {
					t.Fatalf("expected no match, got %q", got.Name)
				}
				return
			}
			if !ok {
				t.Fatalf("expected a match for %s", tc.wantURL)
			}
			if got.URL != tc.wantURL {
				t.Errorf("picked %q (%s), want %s", got.Name, got.URL, tc.wantURL)
			}
		})
	}
}

// Without LinuxOnly, a darwin asset can win — which is why the release-binary
// installer sets it and the deb/AppImage ones don't need to (their suffix
// already constrains the platform).
func TestPickAsset_LinuxOnlyMatters(t *testing.T) {
	assets := fixtureAssets(t)

	got, ok := pickAsset(assets, assetFilter{
		ArchTokens: archTokensFor("arm64"), SkipSidecars: true,
	})
	if !ok {
		t.Fatal("expected a match")
	}
	if got.URL != "https://x/darwin" {
		t.Fatalf("premise changed: without LinuxOnly the darwin asset should win, got %q", got.Name)
	}

	got, ok = pickAsset(assets, assetFilter{
		ArchTokens: archTokensFor("arm64"), SkipSidecars: true, LinuxOnly: true,
	})
	if !ok || got.URL != "https://x/linux-arm64" {
		t.Errorf("LinuxOnly should select the linux asset, got %q", got.Name)
	}
}

func TestArchTokensFor(t *testing.T) {
	cases := map[string][]string{
		"amd64":   {"x86_64", "amd64", "x64"},
		"arm64":   {"aarch64", "arm64"},
		"riscv64": {"riscv64"},
	}
	for goarch, want := range cases {
		got := archTokensFor(goarch)
		if len(got) != len(want) {
			t.Errorf("archTokensFor(%q) = %v, want %v", goarch, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("archTokensFor(%q) = %v, want %v", goarch, got, want)
				break
			}
		}
	}
}

// An empty ArchTokens must not silently match everything in a way that
// surprises callers — it means "any arch", which only the no-arch case uses.
func TestPickAsset_EmptyArchTokensMatchesAny(t *testing.T) {
	assets := []ghAsset{{Name: "plain.deb", URL: "https://x/plain"}}
	got, ok := pickAsset(assets, assetFilter{Suffix: ".deb"})
	if !ok || got.URL != "https://x/plain" {
		t.Errorf("expected the sole .deb, got %v (ok=%v)", got, ok)
	}
}

// The delta release listing, in the order the GitHub API returns it. Two things
// about it broke the delta installer:
//
//  1. the version is embedded in every filename, so the old hand-built URL
//     .../releases/latest/download/git-delta_<arch>.deb could never resolve; and
//  2. the musl build is listed FIRST and also ends in _<arch>.deb, so a filter
//     of {Suffix: ".deb", ArchTokens: ...} alone selects the wrong artifact.
const deltaReleaseFixture = `{"assets":[
  {"name":"git-delta-musl_0.19.2_amd64.deb","browser_download_url":"https://x/delta-musl-amd64"},
  {"name":"git-delta_0.19.2_amd64.deb","browser_download_url":"https://x/delta-amd64"},
  {"name":"git-delta_0.19.2_arm64.deb","browser_download_url":"https://x/delta-arm64"},
  {"name":"git-delta_0.19.2_armhf.deb","browser_download_url":"https://x/delta-armhf"},
  {"name":"git-delta_0.19.2_i386.deb","browser_download_url":"https://x/delta-i386"}
]}`

func deltaAssets(t *testing.T) []ghAsset {
	t.Helper()
	var r struct {
		Assets []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal([]byte(deltaReleaseFixture), &r); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return r.Assets
}

// deltaFilter mirrors the filter installDeltaDeb uses.
func deltaFilter(goarch string) assetFilter {
	return assetFilter{
		Contains:     "git-delta_",
		ArchTokens:   archTokensFor(goarch),
		Suffix:       ".deb",
		SkipSidecars: true,
		LinuxOnly:    true,
	}
}

func TestPickAsset_DeltaRelease(t *testing.T) {
	assets := deltaAssets(t)

	t.Run("amd64 picks the glibc build, not musl", func(t *testing.T) {
		got, ok := pickAsset(assets, deltaFilter("amd64"))
		if !ok {
			t.Fatal("expected a match")
		}
		if got.URL != "https://x/delta-amd64" {
			t.Errorf("picked %q (%s), want https://x/delta-amd64", got.Name, got.URL)
		}
	})

	t.Run("arm64", func(t *testing.T) {
		got, ok := pickAsset(assets, deltaFilter("arm64"))
		if !ok {
			t.Fatal("expected a match")
		}
		if got.URL != "https://x/delta-arm64" {
			t.Errorf("picked %q (%s), want https://x/delta-arm64", got.Name, got.URL)
		}
	})

	// Why Contains is load-bearing. Drop it and the musl build wins on ordering
	// alone. If this ever stops holding, the Contains clause in installDeltaDeb
	// has become dead weight and the comment there is wrong.
	t.Run("without Contains the musl build wins", func(t *testing.T) {
		f := deltaFilter("amd64")
		f.Contains = ""
		got, ok := pickAsset(assets, f)
		if !ok {
			t.Fatal("expected a match")
		}
		if got.URL != "https://x/delta-musl-amd64" {
			t.Fatalf("premise changed: expected musl to win without Contains, got %q", got.Name)
		}
	})

	// The regression itself: no asset is named the way the old URL template
	// assumed, which is why every run 404'd and fell back to the distro package.
	t.Run("no version-less asset name exists", func(t *testing.T) {
		for _, a := range assets {
			if a.Name == "git-delta_amd64.deb" || a.Name == "git-delta_arm64.deb" {
				t.Fatalf("fixture no longer reflects the bug: found %q", a.Name)
			}
		}
	})
}
