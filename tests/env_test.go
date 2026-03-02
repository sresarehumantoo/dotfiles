package tests

import (
	"testing"

	"github.com/owenpierce/dotfiles/src/core"
)

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
