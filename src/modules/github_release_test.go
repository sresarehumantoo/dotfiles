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
