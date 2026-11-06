package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func stampFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []struct {
		rel  string
		mode os.FileMode
		data string
	}{
		{"config/shell/zshrc", 0o644, "zshrc\n"},
		{"config/devtools/tool", 0o700, "#!/bin/sh\n"},
		{"config/git/private", 0o600, "x"},
		{"README.md", 0o644, "outside config, not stamped"},
	}
	for _, f := range files {
		p := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f.data), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, f.mode); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGenerateContentStamp(t *testing.T) {
	root := stampFixture(t)
	st, err := GenerateContentStamp(root, "v2.3.0")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("zshrc\n"))
	want := map[string]ContentFile{
		"config/shell/zshrc":   {SHA256: hex.EncodeToString(sum[:]), Mode: "0644"},
		"config/devtools/tool": {Mode: "0755"},
		"config/git/private":   {Mode: "0644"},
	}
	if st.Version != "v2.3.0" || len(st.Files) != len(want) {
		t.Fatalf("got version %q and %d files, want v2.3.0 and %d: %v", st.Version, len(st.Files), len(want), st.Files)
	}
	for rel, w := range want {
		got, ok := st.Files[rel]
		if !ok {
			t.Errorf("%s not stamped", rel)
			continue
		}
		if got.Mode != w.Mode {
			t.Errorf("%s mode = %s, want %s", rel, got.Mode, w.Mode)
		}
		if w.SHA256 != "" && got.SHA256 != w.SHA256 {
			t.Errorf("%s sha256 = %s, want %s", rel, got.SHA256, w.SHA256)
		}
	}
}

func TestGenerateContentStampRejectsNonRegular(t *testing.T) {
	cases := map[string]func(t *testing.T, p string){
		"symlink": func(t *testing.T, p string) {
			if err := os.Symlink("/etc/passwd", p); err != nil {
				t.Fatal(err)
			}
		},
		"fifo": func(t *testing.T, p string) {
			if err := syscall.Mkfifo(p, 0o644); err != nil {
				t.Skipf("mkfifo: %v", err)
			}
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			root := stampFixture(t)
			mk(t, filepath.Join(root, "config", "shell", "odd"))
			_, err := GenerateContentStamp(root, "v1")
			if err == nil || !strings.Contains(err.Error(), "config/shell/odd") {
				t.Fatalf("err = %v, want a rejection naming config/shell/odd", err)
			}
		})
	}
}

func TestGenerateContentStampNeedsVersionAndFiles(t *testing.T) {
	if _, err := GenerateContentStamp(stampFixture(t), ""); err == nil {
		t.Error("empty version accepted")
	}
	empty := t.TempDir()
	if err := os.Mkdir(filepath.Join(empty, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateContentStamp(empty, "v1"); err == nil {
		t.Error("empty config/ accepted")
	}
}

func TestWriteContentStampIsDeterministic(t *testing.T) {
	root := stampFixture(t)
	var outs [][]byte
	for range 2 {
		st, err := GenerateContentStamp(root, "v1")
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(t.TempDir(), ContentStampName)
		if err := WriteContentStamp(p, st); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		outs = append(outs, b)
	}
	if !bytes.Equal(outs[0], outs[1]) {
		t.Errorf("two writes differ:\n%s\n%s", outs[0], outs[1])
	}
}
