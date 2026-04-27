package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

func TestParseOsRelease(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    core.Distro
	}{
		{
			name:    "SteamOS",
			content: "ID=steamos\nID_LIKE=arch\nVERSION_CODENAME=holo",
			want:    core.DistroSteamOS,
		},
		{
			name:    "Arch Linux",
			content: "ID=arch\n",
			want:    core.DistroArch,
		},
		{
			name:    "Manjaro",
			content: "ID=manjaro\nID_LIKE=arch\n",
			want:    core.DistroArch,
		},
		{
			name:    "Debian",
			content: "ID=debian\nVERSION_CODENAME=bookworm\n",
			want:    core.DistroDebian,
		},
		{
			name:    "Ubuntu",
			content: "ID=ubuntu\nID_LIKE=debian\n",
			want:    core.DistroDebian,
		},
		{
			name:    "Fedora",
			content: "ID=fedora\n",
			want:    core.DistroFedora,
		},
		{
			name:    "Rocky via ID_LIKE",
			content: "ID=rocky\nID_LIKE=\"rhel centos fedora\"\n",
			want:    core.DistroFedora,
		},
		{
			name:    "unknown distro with arch-like",
			content: "ID=garuda\nID_LIKE=arch\n",
			want:    core.DistroArch,
		},
		{
			name:    "unknown distro with debian-like",
			content: "ID=pop\nID_LIKE=\"ubuntu debian\"\n",
			want:    core.DistroDebian,
		},
		{
			name:    "ParrotOS",
			content: "ID=parrot\nID_LIKE=debian\nVERSION_CODENAME=lory\nDEBIAN_CODENAME=bookworm\n",
			want:    core.DistroDebian,
		},
		{
			name:    "ParrotOS Security Edition",
			content: "ID=parrotsec\nID_LIKE=debian\n",
			want:    core.DistroDebian,
		},
		{
			name:    "empty content",
			content: "",
			want:    core.DistroUnknown,
		},
		{
			name:    "no ID field",
			content: "NAME=SomeOS\nVERSION=1.0\n",
			want:    core.DistroUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.ParseOsRelease(tt.content)
			if got != tt.want {
				t.Errorf("ParseOsRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUpstreamDebianCodename(t *testing.T) {
	tests := []struct {
		name          string
		osRelease     string
		debianVersion string
		want          string
	}{
		{
			name:      "real Debian uses VERSION_CODENAME",
			osRelease: "ID=debian\nVERSION_CODENAME=bookworm\n",
			want:      "bookworm",
		},
		{
			name:      "Ubuntu uses VERSION_CODENAME",
			osRelease: "ID=ubuntu\nVERSION_CODENAME=jammy\n",
			want:      "jammy",
		},
		{
			name:      "ParrotOS prefers DEBIAN_CODENAME over VERSION_CODENAME",
			osRelease: "ID=parrot\nID_LIKE=debian\nVERSION_CODENAME=lory\nDEBIAN_CODENAME=bookworm\n",
			want:      "bookworm",
		},
		{
			name:          "ParrotOS without DEBIAN_CODENAME falls back to /etc/debian_version",
			osRelease:     "ID=parrot\nID_LIKE=debian\nVERSION_CODENAME=lory\n",
			debianVersion: "12.4\n",
			want:          "bookworm",
		},
		{
			name:          "Kali rolling — VERSION_CODENAME is unrecognized, debian_version unrecognized too, fall back to default",
			osRelease:     "ID=kali\nID_LIKE=debian\nVERSION_CODENAME=kali-rolling\n",
			debianVersion: "kali-rolling\n",
			want:          "bookworm",
		},
		{
			name:          "Mint Cinnamon (Ubuntu-based) with Ubuntu codename in VERSION_CODENAME",
			osRelease:     "ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_CODENAME=jammy\n",
			debianVersion: "",
			want:          "jammy",
		},
		{
			name:          "debian_version with major.minor splits correctly",
			osRelease:     "ID=parrot\n",
			debianVersion: "11.7",
			want:          "bullseye",
		},
		{
			name:          "debian_version with trixie/sid string is unrecognized",
			osRelease:     "",
			debianVersion: "trixie/sid",
			want:          "bookworm",
		},
		{
			name:      "DEBIAN_CODENAME with garbage is ignored, falls through",
			osRelease: "DEBIAN_CODENAME=notarealcodename\nVERSION_CODENAME=bullseye\n",
			want:      "bullseye",
		},
		{
			name:      "empty everything returns default",
			osRelease: "",
			want:      "bookworm",
		},
		{
			name:      "DEBIAN_CODENAME quoted",
			osRelease: "DEBIAN_CODENAME=\"trixie\"\n",
			want:      "trixie",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.ParseUpstreamDebianCodename(tt.osRelease, tt.debianVersion)
			if got != tt.want {
				t.Errorf("ParseUpstreamDebianCodename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseProcVersion_WSL(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "WSL2",
			content: "Linux version 5.15.90.1-microsoft-standard-WSL2 (root@1234) (gcc) #1 SMP",
			want:    true,
		},
		{
			name:    "WSL1",
			content: "Linux version 4.4.0-19041-Microsoft (Microsoft@Microsoft) (gcc version 5.4.0)",
			want:    true,
		},
		{
			name:    "native Linux",
			content: "Linux version 6.1.0-17-amd64 (debian-kernel@lists.debian.org)",
			want:    false,
		},
		{
			name:    "empty",
			content: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.ParseProcVersion(tt.content)
			if got != tt.want {
				t.Errorf("ParseProcVersion(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
