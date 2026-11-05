package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ContentStampName is the file at the root of packaged content that lists every
// file under config/ with its digest and mode.
const ContentStampName = ".dfinstall-content.json"

// ContentStamp identifies one packaged copy of the config/ tree. File keys are
// slash-separated paths relative to the content root, such as
// "config/shell/zshrc".
type ContentStamp struct {
	Version string                 `json:"version"`
	Files   map[string]ContentFile `json:"files"`
}

// ContentFile is one stamped file. Mode is "0755" or "0644": only the
// executable bit is meaningful, so other permission bits are not recorded.
type ContentFile struct {
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

// GenerateContentStamp walks root/config and stamps every regular file. A
// symlink or any other non-regular entry is an error, since packaged content
// is extracted through an allow-list that admits only files and directories.
func GenerateContentStamp(root, version string) (ContentStamp, error) {
	st := ContentStamp{Version: version, Files: map[string]ContentFile{}}
	if version == "" {
		return st, fmt.Errorf("content stamp needs a version")
	}
	err := filepath.WalkDir(filepath.Join(root, "config"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular file (%s)", rel, d.Type())
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		mode := "0644"
		if info.Mode().Perm()&0o111 != 0 {
			mode = "0755"
		}
		st.Files[rel] = ContentFile{SHA256: sum, Mode: mode}
		return nil
	})
	if err != nil {
		return st, err
	}
	if len(st.Files) == 0 {
		return st, fmt.Errorf("no files under %s", filepath.Join(root, "config"))
	}
	return st, nil
}

// WriteContentStamp writes st as indented JSON. Map keys marshal sorted, so the
// same tree always produces the same bytes.
func WriteContentStamp(path string, st ContentStamp) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
