package tests

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func TestContainsSudoInvocation(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		args []string
		want bool
	}{
		{
			name: "direct sudo command",
			cmd:  "sudo",
			args: []string{"apt-get", "update"},
			want: true,
		},
		{
			name: "sudo as positional arg",
			cmd:  "env",
			args: []string{"FOO=bar", "sudo", "ls"},
			want: true,
		},
		{
			name: "bash -c with embedded sudo (the bug we care about)",
			cmd:  "bash",
			args: []string{"-c", "curl -fsSL example.com | sudo tee /etc/foo.list"},
			want: true,
		},
		{
			name: "bash -c with embedded sudo + heredoc",
			cmd:  "bash",
			args: []string{"-c", "cat <<'EOF' | sudo tee /etc/foo\nbar\nEOF"},
			want: true,
		},
		{
			name: "no sudo anywhere",
			cmd:  "curl",
			args: []string{"-fsSL", "https://example.com"},
			want: false,
		},
		{
			name: "false positive guard — substring without space",
			cmd:  "make",
			args: []string{"presudoku"},
			want: false,
		},
		{
			name: "no args",
			cmd:  "ls",
			args: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modules.ContainsSudoInvocation(tt.cmd, tt.args)
			if got != tt.want {
				t.Errorf("ContainsSudoInvocation(%q, %v) = %v, want %v", tt.cmd, tt.args, got, tt.want)
			}
		})
	}
}
