package modules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// managedJSON is a settings file that its program also rewrites, so it is
// merged rather than linked: a symlink would either be written through into
// this repo or replaced by a plain file. Only the top-level keys of the
// tracked base are managed; every other key in the live file is left alone.
type managedJSON struct {
	label string // for messages, e.g. "Claude settings"
	base  string // tracked file in this repo
	live  string // the program's own file
}

// state returns the tracked base, the live settings (empty when the file does
// not exist yet) and the managed keys whose live value differs.
func (f managedJSON) state() (base, live map[string]any, stale []string, err error) {
	base, err = readJSONObject(f.base)
	if err != nil {
		return nil, nil, nil, err
	}
	live, err = readJSONObject(f.live)
	if errors.Is(err, fs.ErrNotExist) {
		live, err = map[string]any{}, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	for k, v := range base {
		if lv, ok := live[k]; !ok || !reflect.DeepEqual(lv, v) {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	return base, live, stale, nil
}

// statusExtra summarizes the live file for a Status line. A count, not the
// names: the list can run past a terminal's width, and diff names each key.
func (f managedJSON) statusExtra() string {
	name := filepath.Base(f.live)
	if _, err := os.Stat(f.live); errors.Is(err, fs.ErrNotExist) {
		return name + " not merged yet"
	}
	_, _, stale, err := f.state()
	switch {
	case err != nil:
		return name + " unreadable"
	case len(stale) == 1:
		return name + ": 1 managed key differs"
	case len(stale) > 1:
		return fmt.Sprintf("%s: %d managed keys differ", name, len(stale))
	}
	return ""
}

// drift reports managed keys that something else changed, typically the
// program's own settings UI.
func (f managedJSON) drift() []DiffProblem {
	if _, err := os.Stat(f.live); errors.Is(err, fs.ErrNotExist) {
		return nil // never installed here; Status already says so
	}
	_, _, stale, err := f.state()
	switch {
	case err != nil:
		return []DiffProblem{{Dst: f.live, State: "content", Detail: "unreadable"}}
	case len(stale) > 0:
		return []DiffProblem{{Dst: f.live, State: "content", Detail: "managed keys differ: " + strings.Join(stale, ", ")}}
	}
	return nil
}

func (f managedJSON) merge() error {
	path := f.live
	if err := core.CheckTarget(path); err != nil {
		return err
	}
	base, live, stale, err := f.state()
	if err != nil {
		// Never replace a file we could not parse: it holds settings we don't own.
		return fmt.Errorf("not merging %s: %w", f.label, err)
	}
	if len(stale) == 0 {
		core.Ok("%s already up to date", f.label)
		return nil
	}
	if core.DryRun {
		core.Info("would set %s in %s", strings.Join(stale, ", "), path)
		return nil
	}

	for _, k := range stale {
		live[k] = base[k]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// The live file holds prose with <, > and &; keep them readable.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(live); err != nil {
		return err
	}

	if err := core.BackupFile(path); err != nil {
		core.Warn("backup failed for %s: %v", path, err)
	}
	perm := fs.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := core.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	core.Changed("%s: set %s", f.label, strings.Join(stale, ", "))
	return nil
}

// readJSONObject decodes a JSON object with UseNumber so integers round-trip
// exactly through a rewrite.
func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m == nil {
		return nil, fmt.Errorf("%s: not a JSON object", path)
	}
	return m, nil
}
