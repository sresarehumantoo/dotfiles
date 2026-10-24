package tests

import (
	"strings"
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
	"github.com/sresarehumantoo/dotfiles/v2/internal/modules"
)

func TestFormatStatusLine(t *testing.T) {
	tests := []struct {
		name string
		row  modules.StatusRow
		want []string
		not  []string
	}{
		{
			name: "installed",
			row:  modules.StatusRow{ModuleStatus: core.ModuleStatus{Name: "shell", Linked: 5}, State: modules.StateInstalled},
			want: []string{"shell", "5/5"},
		},
		{
			name: "partial",
			row:  modules.StatusRow{ModuleStatus: core.ModuleStatus{Name: "nvim", Linked: 10, Missing: 7}, State: modules.StatePartial},
			want: []string{"nvim", "10/17"},
		},
		{
			name: "with extra info",
			row:  modules.StatusRow{ModuleStatus: core.ModuleStatus{Name: "delta", Linked: 1, Extra: "installed"}, State: modules.StateInstalled},
			want: []string{"delta", "1/1", "installed"},
		},
		{
			name: "unavailable shows the reason and no count",
			row:  modules.StatusRow{ModuleStatus: core.ModuleStatus{Name: "wsl"}, State: modules.StateUnavailable, Reason: "WSL only"},
			want: []string{"wsl", "WSL only"},
			not:  []string{"0/0"},
		},
		{
			name: "skipped shows the reason, not the module's own note",
			row:  modules.StatusRow{ModuleStatus: core.ModuleStatus{Name: "windev", Extra: "disabled"}, State: modules.StateSkipped, Reason: "opt-in: dfinstall install windev"},
			want: []string{"windev", "opt-in"},
			not:  []string{"disabled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modules.FormatStatusLine(tt.row)
			for _, s := range tt.want {
				if !strings.Contains(got, s) {
					t.Errorf("FormatStatusLine() = %q, missing %q", got, s)
				}
			}
			for _, s := range tt.not {
				if strings.Contains(got, s) {
					t.Errorf("FormatStatusLine() = %q, should not contain %q", got, s)
				}
			}
		})
	}
}
