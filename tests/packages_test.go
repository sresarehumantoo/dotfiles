package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestResolvePkgs(t *testing.T) {
	tests := []struct {
		name string
		mgr  string
		pkgs []string
		want []string
	}{
		{
			name: "apt passthrough",
			mgr:  "apt-get",
			pkgs: []string{"fd-find", "bat", "build-essential"},
			want: []string{"fd-find", "bat", "build-essential"},
		},
		{
			name: "pacman renames",
			mgr:  "pacman",
			pkgs: []string{"fd-find", "build-essential", "golang"},
			want: []string{"fd", "base-devel", "go"},
		},
		{
			name: "pacman skips empty mappings",
			mgr:  "pacman",
			pkgs: []string{"python3-venv", "neovim"},
			want: []string{"neovim"},
		},
		{
			name: "pacman unknown package passthrough",
			mgr:  "pacman",
			pkgs: []string{"htop", "curl"},
			want: []string{"htop", "curl"},
		},
		{
			name: "pacman docker packages",
			mgr:  "pacman",
			pkgs: []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"},
			want: []string{"docker", "docker-buildx", "docker-compose"},
		},
		{
			name: "empty input",
			mgr:  "pacman",
			pkgs: []string{},
			want: []string{},
		},
		{
			name: "brew passthrough",
			mgr:  "brew",
			pkgs: []string{"fd-find", "bat"},
			want: []string{"fd-find", "bat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modules.ResolvePkgs(tt.mgr, tt.pkgs)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ResolvePkgs(%q, %v) = %v (len %d), want %v (len %d)",
					tt.mgr, tt.pkgs, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ResolvePkgs(%q, %v)[%d] = %q, want %q",
						tt.mgr, tt.pkgs, i, got[i], tt.want[i])
				}
			}
		})
	}
}
