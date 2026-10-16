package modules

import (
	"io/fs"
	"path/filepath"
	"testing"
)

// A file added under config/nvim does nothing until nvimLinks names it, and
// nothing else reports the omission: a new plugin spec simply never loads.
func TestNvimLinksCoverConfigDir(t *testing.T) {
	linked := map[string]bool{}
	for _, l := range nvimLinks {
		linked[l.Src] = true
	}
	root := filepath.Join("..", "..", "config")
	err := filepath.WalkDir(filepath.Join(root, "nvim"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch rel {
		case "nvim/windev/windev.lua": // linked by the windev module
			return nil
		}
		if !linked[filepath.ToSlash(rel)] {
			t.Errorf("config/%s is not in nvimLinks, so it is never installed", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
