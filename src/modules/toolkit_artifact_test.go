package modules

// In-package: artifactFor and toolArtifact are unexported.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestArtifactFor_LocationsByMethod(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		method   string
		binary   string
		wantPath string
		wantDir  bool
		wantBin  string
	}{
		{"appimage", "obsidian", filepath.Join(home, ".local/bin/obsidian.AppImage"), false, ""},
		{"git_clone", "seclists", filepath.Join(home, ".local/share/toolkit/seclists"), true, ""},
		{"release_binary", "chainsaw", filepath.Join(home, ".local/bin/chainsaw"), false, ""},
		{"rustup", "rustup", filepath.Join(home, ".cargo/bin/rustup"), false, ""},
		{"apt", "nmap", "", false, "nmap"},
		{"go", "gobuster", "", false, "gobuster"},
		{"cargo", "rustscan", "", false, "rustscan"},
		{"pipx", "nxc", "", false, "nxc"},
		{"deb", "ghostty", "", false, "ghostty"},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			got := artifactFor(core.RegistryTool{Method: tc.method, Binary: tc.binary})
			if got.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tc.wantPath)
			}
			if got.IsDir != tc.wantDir {
				t.Errorf("IsDir = %v, want %v", got.IsDir, tc.wantDir)
			}
			if got.Bin != tc.wantBin {
				t.Errorf("Bin = %q, want %q", got.Bin, tc.wantBin)
			}
		})
	}
}

// Status, Uninstall and the menu all ask artifactFor where a tool lives, and
// Uninstall removes what it's told. If the path-producing helper and the
// presence check ever disagreed, uninstall would delete the wrong thing or
// silently skip. Round-trip each method through both.
func TestArtifactFor_InstalledMatchesTheReportedPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, method := range []string{"appimage", "git_clone", "release_binary", "rustup"} {
		t.Run(method, func(t *testing.T) {
			art := artifactFor(core.RegistryTool{Method: method, Binary: "thing"})

			if art.Installed() {
				t.Fatalf("reports installed before anything exists at %s", art.Path)
			}

			if art.IsDir {
				if err := os.MkdirAll(art.Path, 0755); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(art.Path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(art.Path, []byte("x"), 0755); err != nil {
					t.Fatal(err)
				}
			}

			if !art.Installed() {
				t.Errorf("created %s but Installed() is false", art.Path)
			}
		})
	}
}

// git_clone artifacts are directories; a stray file at the same path must not
// count as installed, or uninstall would try to RemoveAll a regular file.
func TestArtifactFor_GitCloneRequiresDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	art := artifactFor(core.RegistryTool{Method: "git_clone", Binary: "seclists"})
	if err := os.MkdirAll(filepath.Dir(art.Path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(art.Path, []byte("not a clone"), 0644); err != nil {
		t.Fatal(err)
	}

	if art.Installed() {
		t.Error("a regular file at the clone path counted as installed")
	}
}

// The installers write to artifactForMethod(...).Path, and Status/Uninstall
// read artifactFor(...).Path. They must be the same string.
func TestArtifactForMethod_AgreesWithArtifactFor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, method := range []string{"appimage", "git_clone", "release_binary", "rustup", "apt"} {
		a := artifactFor(core.RegistryTool{Method: method, Binary: "thing"})
		b := artifactForMethod(method, "thing")
		if a != b {
			t.Errorf("%s: artifactFor=%+v artifactForMethod=%+v", method, a, b)
		}
	}
}

// An unknown method must fall back to a PATH lookup rather than producing an
// empty path that a caller might then act on.
func TestArtifactFor_UnknownMethodFallsBackToPath(t *testing.T) {
	art := artifactFor(core.RegistryTool{Method: "snap", Binary: "thing"})
	if art.Path != "" || art.Bin != "thing" {
		t.Errorf("unknown method gave %+v, want a PATH lookup", art)
	}
}
