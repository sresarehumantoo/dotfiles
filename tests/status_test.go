package tests

import (
	"strings"
	"testing"

	"github.com/owenpierce/dotfiles/src/core"
	"github.com/owenpierce/dotfiles/src/modules"
)

func TestFormatStatusLine(t *testing.T) {
	tests := []struct {
		name   string
		status core.ModuleStatus
		want   []string // substrings that should be present
	}{
		{
			name:   "all linked",
			status: core.ModuleStatus{Name: "shell", Linked: 5, Missing: 0},
			want:   []string{"shell", "5", "0"},
		},
		{
			name:   "some missing",
			status: core.ModuleStatus{Name: "nvim", Linked: 10, Missing: 7},
			want:   []string{"nvim", "10", "7"},
		},
		{
			name:   "with extra info",
			status: core.ModuleStatus{Name: "delta", Linked: 1, Missing: 0, Extra: "installed"},
			want:   []string{"delta", "1", "0", "installed"},
		},
		{
			name:   "not WSL",
			status: core.ModuleStatus{Name: "wsl", Linked: 0, Missing: 0, Extra: "not WSL"},
			want:   []string{"wsl", "0", "not WSL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modules.FormatStatusLine(tt.status)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Errorf("FormatStatusLine() = %q, missing substring %q", got, substr)
				}
			}
		})
	}
}
