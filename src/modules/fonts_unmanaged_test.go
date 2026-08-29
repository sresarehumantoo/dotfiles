package modules

import (
	"reflect"
	"testing"
)

func TestUnmanagedPaths(t *testing.T) {
	const managed = "/home/u/.local/share/fonts/IosevkaTerm"

	tests := []struct {
		name string
		out  string
		want []string
	}{
		{
			name: "everything managed",
			out: managed + "/IosevkaTermNerdFont-Regular.ttf: \n" +
				managed + "/IosevkaTermNerdFont-Bold.ttf: \n",
			want: nil,
		},
		{
			// The state this check exists for: a hand-installed copy sitting
			// flat beside the managed directory.
			name: "a flat duplicate beside the managed dir",
			out: managed + "/IosevkaTermNerdFont-Regular.ttf: \n" +
				"/home/u/.local/share/fonts/IosevkaTermNerdFont-Regular.ttf: \n",
			want: []string{"/home/u/.local/share/fonts/IosevkaTermNerdFont-Regular.ttf"},
		},
		{
			name: "a system-wide copy",
			out:  "/usr/share/fonts/truetype/IosevkaTermNerdFont-Regular.ttf: \n",
			want: []string{"/usr/share/fonts/truetype/IosevkaTermNerdFont-Regular.ttf"},
		},
		{
			// ⚠ The prefix test needs the separator. A sibling directory whose
			// name merely starts with the managed one must NOT read as managed,
			// or its faces are silently never reported.
			name: "a sibling directory sharing the name prefix",
			out:  managed + "Propo/IosevkaTermNerdFontPropo-Regular.ttf: \n",
			want: []string{managed + "Propo/IosevkaTermNerdFontPropo-Regular.ttf"},
		},
		{
			name: "output is sorted, blank lines ignored",
			out:  "/b/two.ttf: \n\n/a/one.ttf: \n   \n",
			want: []string{"/a/one.ttf", "/b/two.ttf"},
		},
		{
			name: "no fonts at all",
			out:  "",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unmanagedPaths(tc.out, managed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A machine with no fontconfig must not be told its fonts are duplicated: a
// missing tool is not evidence. Same lesson as the old fc-list gate, which read
// "absent" from a missing binary and re-downloaded 28 MB every run.
func TestUnmanagedFontCopies_NoManagedDirIsSilent(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	if got := unmanagedFontCopies(iosevkaTerm); got != nil {
		t.Errorf("with no resolvable font dir, want nil, got %v", got)
	}
}
