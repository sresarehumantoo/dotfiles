package modules

// In-package: pickAsset and assetFilter are unexported. This selection logic
// previously lived inline in three installers, each reachable only by actually
// hitting the GitHub API, so none of it was covered.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// The delta 0.20.1 release listing, in the order the GitHub API returns it:
// .debs, tarballs for several targets, and other platforms' builds.
var deltaRelease = []string{
	"delta-0.20.1-aarch64-apple-darwin.tar.gz",
	"delta-0.20.1-aarch64-unknown-linux-gnu.tar.gz",
	"delta-0.20.1-arm-unknown-linux-gnueabihf.tar.gz",
	"delta-0.20.1-i686-unknown-linux-gnu.tar.gz",
	"delta-0.20.1-x86_64-apple-darwin.tar.gz",
	"delta-0.20.1-x86_64-pc-windows-msvc.zip",
	"delta-0.20.1-x86_64-unknown-linux-gnu.tar.gz",
	"delta-0.20.1-x86_64-unknown-linux-musl.tar.gz",
	"git-delta-musl_0.20.1_amd64.deb",
	"git-delta_0.20.1_amd64.deb",
	"git-delta_0.20.1_arm64.deb",
}

func TestDeltaReleaseAsset(t *testing.T) {
	var assets []ghAsset
	for _, n := range deltaRelease {
		assets = append(assets, ghAsset{Name: n})
	}
	cases := map[string]string{
		// musl is static, so it runs regardless of the host's glibc.
		"amd64": "delta-0.20.1-x86_64-unknown-linux-musl.tar.gz",
		// No aarch64 musl build is published; glibc is the fallback.
		"arm64": "delta-0.20.1-aarch64-unknown-linux-gnu.tar.gz",
		"arm":   "delta-0.20.1-arm-unknown-linux-gnueabihf.tar.gz",
	}
	for goarch, want := range cases {
		got, ok := deltaReleaseAsset(assets, goarch)
		if !ok || got.Name != want {
			t.Errorf("%s: got %q (ok=%v), want %s", goarch, got.Name, ok, want)
		}
	}
	if got, ok := deltaReleaseAsset(assets, "riscv64"); ok {
		t.Errorf("riscv64: picked %s, want no match", got.Name)
	}
}

func TestPickAsset_DarwinOnly(t *testing.T) {
	assets := fixtureAssets(t)

	got, ok := pickAsset(assets, assetFilter{
		ArchTokens: archTokensFor("arm64"), SkipSidecars: true, DarwinOnly: true,
	})
	if !ok || got.URL != "https://x/darwin" {
		t.Errorf("DarwinOnly arm64 picked %q, want the darwin asset", got.Name)
	}

	if got, ok := pickAsset(assets, assetFilter{
		ArchTokens: archTokensFor("amd64"), SkipSidecars: true, DarwinOnly: true,
	}); ok {
		t.Errorf("no darwin amd64 build exists, but DarwinOnly picked %q", got.Name)
	}

	universal := []ghAsset{
		{Name: "tool-linux-x86_64.tar.gz", URL: "https://x/linux"},
		{Name: "tool-macos-universal.zip", URL: "https://x/universal"},
	}
	got, ok = pickAsset(universal, assetFilter{
		ArchTokens: append(archTokensFor("arm64"), "universal"), DarwinOnly: true,
	})
	if !ok || got.URL != "https://x/universal" {
		t.Errorf("universal build not picked, got %q", got.Name)
	}
}

// The tree-sitter v0.27.1 asset list. Its 32-bit builds are named "arm" and
// "x86", which must not satisfy the arm64 and amd64 tokens.
func TestTreeSitterAsset(t *testing.T) {
	var assets []ghAsset
	for _, n := range []string{
		"tree-sitter-cli-linux-arm.zip", "tree-sitter-cli-linux-arm64.zip", "tree-sitter-cli-linux-x64.zip",
		"tree-sitter-cli-linux-x86.zip", "tree-sitter-cli-macos-arm64.zip", "tree-sitter-cli-windows-x64.zip",
		"tree-sitter-linux-arm.gz", "tree-sitter-linux-arm64.gz", "tree-sitter-linux-powerpc64.gz",
		"tree-sitter-linux-x64.gz", "tree-sitter-linux-x86.gz", "tree-sitter-macos-arm64.gz",
		"tree-sitter-macos-x64.gz", "tree-sitter-windows-x64.gz", "web-tree-sitter.tar.gz",
	} {
		assets = append(assets, ghAsset{Name: n, URL: "https://x/" + n})
	}
	for goarch, want := range map[string]string{
		"amd64": "tree-sitter-linux-x64.gz",
		"arm64": "tree-sitter-linux-arm64.gz",
	} {
		got, ok := pickAsset(assets, treeSitterFilter(goarch))
		if !ok || got.Name != want {
			t.Errorf("%s: got %q (ok=%v), want %q", goarch, got.Name, ok, want)
		}
	}
}

func TestDownloadVerified(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "asset.deb")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sum = "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"
	url := "file://" + src

	cases := []struct {
		name    string
		digest  string
		wantErr string
	}{
		{"match", "sha256:" + sum, ""},
		{"uppercase hex", "sha256:" + strings.ToUpper(sum), ""},
		{"mismatch", "sha256:" + strings.Repeat("0", 64), "sha256 mismatch"},
		{"no digest", "", "no published sha256 digest"},
		{"other algorithm", "sha512:" + sum, "no published sha256 digest"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "out")
			err := downloadVerified(context.Background(), ghAsset{Name: "asset.deb", URL: url, Digest: c.digest}, dst)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, c.wantErr)
			}
		})
	}
}

func TestGhAssetDigestParsed(t *testing.T) {
	var release struct {
		Assets []ghAsset `json:"assets"`
	}
	in := `{"assets":[{"name":"a","browser_download_url":"u","digest":"sha256:abc"},{"name":"b","browser_download_url":"u","digest":null}]}`
	if err := json.Unmarshal([]byte(in), &release); err != nil {
		t.Fatal(err)
	}
	if release.Assets[0].Digest != "sha256:abc" || release.Assets[1].Digest != "" {
		t.Fatalf("digests = %q, %q", release.Assets[0].Digest, release.Assets[1].Digest)
	}
}

// The ghostty-debian 1.3.1+6 release as published: debug-symbol packages
// listed first, then one build per distro release, among source artifacts.
var ghosttyDebianRelease = []string{
	"ghostty-dbgsym_1.3.1-6.forky_amd64.deb",
	"ghostty-dbgsym_1.3.1-6.questing_amd64.deb",
	"ghostty-dbgsym_1.3.1-6.resolute_amd64.deb",
	"ghostty-dbgsym_1.3.1-6.trixie_amd64.deb",
	"ghostty_1.3.1-6.forky.debian.tar.xz",
	"ghostty_1.3.1-6.forky.dsc",
	"ghostty_1.3.1-6.forky_amd64.deb",
	"ghostty_1.3.1-6.noble_amd64.deb",
	"ghostty_1.3.1-6.questing_amd64.deb",
	"ghostty_1.3.1-6.resolute_amd64.deb",
	"ghostty_1.3.1-6.sid.dsc",
	"ghostty_1.3.1-6.trixie.debian.tar.xz",
	"ghostty_1.3.1-6.trixie_amd64.deb",
	"ghostty_1.3.1.orig.tar.gz",
}

func TestPickDebForDistro(t *testing.T) {
	var assets []ghAsset
	for _, n := range ghosttyDebianRelease {
		assets = append(assets, ghAsset{Name: n})
	}
	candidates := filterAssets(assets, assetFilter{
		ArchTokens: archTokensFor("amd64"),
		Suffix:     ".deb",
		Exclude:    debugPackageMarkers,
	})
	for _, c := range candidates {
		if strings.Contains(c.Name, "dbgsym") {
			t.Fatalf("debug package survived the filter: %s", c.Name)
		}
	}

	cases := []struct {
		name      string
		codenames []string
		want      string
	}{
		{"debian trixie", []string{"trixie", "trixie"}, "ghostty_1.3.1-6.trixie_amd64.deb"},
		{"ubuntu resolute is not an upstream codename", []string{"resolute", "trixie"}, "ghostty_1.3.1-6.resolute_amd64.deb"},
		{"ubuntu noble", []string{"noble", "noble"}, "ghostty_1.3.1-6.noble_amd64.deb"},
		{"kali falls back to upstream", []string{"kali-rolling", "trixie"}, "ghostty_1.3.1-6.trixie_amd64.deb"},
		{"sid has no binary build", []string{"sid", "sid"}, ""},
		{"bookworm has no build", []string{"bookworm", "bookworm"}, ""},
		{"no codename known", []string{"", ""}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := pickDebForDistro(candidates, c.codenames)
			if c.want == "" {
				if err == nil {
					t.Fatalf("picked %s, want an error", got.Name)
				}
				return
			}
			if err != nil || got.Name != c.want {
				t.Fatalf("got %q, %v; want %s", got.Name, err, c.want)
			}
		})
	}
}

func TestPickDebForDistro_SingleDebNamesNoDistro(t *testing.T) {
	got, err := pickDebForDistro([]ghAsset{{Name: "tool_1.0_amd64.deb"}}, []string{"trixie"})
	if err != nil || got.Name != "tool_1.0_amd64.deb" {
		t.Fatalf("got %q, %v", got.Name, err)
	}
}

func TestSameRelease(t *testing.T) {
	cases := []struct {
		out, tag string
		want     bool
	}{
		{"delta 0.18.2\n", "0.18.2", true},
		{"tree-sitter 0.26.3 (abc123)\n", "v0.26.3", true},
		{"Terraform v1.13.4\non linux_amd64\n", "1.13.4", true},
		{"NVIM v0.12.1\nBuild type: Release\n", "v0.12.1", true},
		{"delta 0.18.1\n", "0.18.2", false},
		{"tree-sitter 0.26.3\n", "v0.26.30", false},
		{"", "0.18.2", false},
		{"delta 0.18.2\n", "", false},
	}
	for _, c := range cases {
		if got := sameRelease([]byte(c.out), c.tag); got != c.want {
			t.Errorf("sameRelease(%q, %q) = %v, want %v", c.out, c.tag, got, c.want)
		}
	}
}
