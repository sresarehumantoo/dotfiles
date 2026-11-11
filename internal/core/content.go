package core

import (
	"archive/tar"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Packaged content is installed into a store of read-only copies under
// ~/.local/share/dfinstall/content: one directory per version and a
// "current" symlink naming the live one. The canonical pointer names
// content/current, so managed links never change when a new version is
// swapped in. Link targets must keep the unresolved path: resolving current
// would pin every link to one version.

// Limits for what one content source may write.
const (
	maxContentFiles    = 10000
	maxContentFileSize = 32 << 20
	maxContentBytes    = 256 << 20
)

const contentCurrentName = "current"

// ContentStoreDir is where packaged content versions are kept.
func ContentStoreDir() string { return AppDataPath("content") }

// ContentCurrentDir is the path the canonical pointer names in packaged mode.
func ContentCurrentDir() string { return SubPath(ContentStoreDir(), contentCurrentName) }

// PackagedContentDir returns the content a deb or Homebrew install ships
// beside its binary (/usr/share/dfinstall, or the keg's share/dfinstall), or ""
// when this binary has none.
func PackagedContentDir() string {
	switch BuildChannel() {
	case ChannelDeb, ChannelBrew:
	default:
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return contentDirBeside(exe)
}

// contentDirBeside resolves exe (a Homebrew bin/ entry is a symlink into the
// keg) and looks for a stamped tree at ../share/dfinstall. Resolving is safe
// because the result is only read from, never linked to.
func contentDirBeside(exe string) string {
	exe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "share", AppDirName)
	if !fileExists(filepath.Join(dir, ContentStampName)) {
		return ""
	}
	return dir
}

// ReadContentStamp reads and validates the stamp at the root of a content
// tree. Every key must be a clean path under config/, since the stamp decides
// what a verified tree may contain.
func ReadContentStamp(root string) (ContentStamp, error) {
	var st ContentStamp
	data, err := os.ReadFile(filepath.Join(root, ContentStampName))
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("parse %s: %w", ContentStampName, err)
	}
	if err := validContentVersion(st.Version); err != nil {
		return st, fmt.Errorf("%s: %w", ContentStampName, err)
	}
	if len(st.Files) == 0 {
		return st, fmt.Errorf("%s lists no files", ContentStampName)
	}
	if len(st.Files) > maxContentFiles {
		return st, fmt.Errorf("%s lists %d files, over the %d limit", ContentStampName, len(st.Files), maxContentFiles)
	}
	for rel, f := range st.Files {
		if !validContentPath(rel) || rel == "config" {
			return st, fmt.Errorf("%s: invalid path %q", ContentStampName, rel)
		}
		if f.Mode != "0644" && f.Mode != "0755" {
			return st, fmt.Errorf("%s: %s has mode %q", ContentStampName, rel, f.Mode)
		}
		if b, err := hex.DecodeString(f.SHA256); err != nil || len(b) != 32 {
			return st, fmt.Errorf("%s: %s has no valid sha256", ContentStampName, rel)
		}
	}
	return st, nil
}

// ContentUpToDate reports whether current names version and that copy still
// matches its stamp.
func ContentUpToDate(version string) bool {
	store := ContentStoreDir()
	return store != "" && currentContentVersion(store) == version &&
		verifyContent(filepath.Join(store, version), version) == nil
}

// SyncContentDir installs the content tree at src (a package's
// share/dfinstall) as version and makes it current. src is only read.
func (s *InstallSession) SyncContentDir(src, version string) error {
	st, err := ReadContentStamp(src)
	if err != nil {
		return fmt.Errorf("packaged content at %s: %w", src, err)
	}
	if st.Version != version {
		return fmt.Errorf("packaged content at %s is %s but dfinstall is %s; reinstall the package", src, st.Version, version)
	}
	return s.syncContent(version, src, func(stage string) error { return copyContentTree(src, stage) })
}

// SyncContentTarball installs a release's content archive as version and
// makes it current. The archive must already be digest-verified.
func (s *InstallSession) SyncContentTarball(archive, version string) error {
	return s.syncContent(version, archive, func(stage string) error { return extractContentTarball(archive, stage) })
}

// syncContent stages a copy beside the store's versions, verifies it against
// its stamp, seals it read-only, renames it into place and swaps current.
// Each step leaves a usable store if the next never runs: the swap is a
// rename, and a sealed version directory left by an interrupted run is
// verified and reused.
func (s *InstallSession) syncContent(version, from string, stage func(dir string) error) error {
	if s == nil || s.finished {
		return errors.New("content sync must run inside an install session")
	}
	if err := validContentVersion(version); err != nil {
		return err
	}
	store := ContentStoreDir()
	if err := CheckTarget(store); err != nil {
		return err
	}
	dst := filepath.Join(store, version)
	prev := currentContentVersion(store)
	ready := verifyContent(dst, version) == nil
	if prev == version && ready {
		return nil
	}
	if DryRun {
		Changed("would install content %s from %s", version, from)
		return nil
	}

	if err := os.MkdirAll(store, 0o755); err != nil {
		return err
	}
	if !ready {
		tmp := dst + ".tmp"
		if err := removeStoreDir(tmp); err != nil {
			return err
		}
		if err := os.Mkdir(tmp, 0o755); err != nil {
			return err
		}
		if err := stage(tmp); err != nil {
			_ = removeStoreDir(tmp)
			return fmt.Errorf("content %s from %s: %w", version, from, err)
		}
		if err := verifyContent(tmp, version); err != nil {
			_ = removeStoreDir(tmp)
			return fmt.Errorf("content %s from %s: %w", version, from, err)
		}
		if err := sealContent(tmp); err != nil {
			_ = removeStoreDir(tmp)
			return err
		}
		if err := removeStoreDir(dst); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
	}

	link := filepath.Join(store, contentCurrentName+".tmp")
	if err := os.Remove(link); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Symlink(version, link); err != nil {
		return err
	}
	if err := os.Rename(link, filepath.Join(store, contentCurrentName)); err != nil {
		_ = os.Remove(link)
		return fmt.Errorf("make content %s current: %w", version, err)
	}
	Changed("installed content %s", version)
	pruneContent(store, version, prev)
	return nil
}

// currentContentVersion is the version directory current names, or "".
func currentContentVersion(store string) string {
	target, err := os.Readlink(filepath.Join(store, contentCurrentName))
	if err != nil || validContentVersion(target) != nil {
		return ""
	}
	return target
}

// pruneContent removes every version directory except current and the one it
// replaced, which is what a rollback to the previous package would use.
// A failure only costs disk, so it warns.
func pruneContent(store string, keep ...string) {
	entries, err := os.ReadDir(store)
	if err != nil {
		Warn("prune content: %v", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() || slices.Contains(keep, e.Name()) {
			continue
		}
		if err := removeStoreDir(filepath.Join(store, e.Name())); err != nil {
			Warn("prune content %s: %v", e.Name(), err)
			continue
		}
		Changed("removed old content %s", e.Name())
	}
}

// verifyContent checks that root holds exactly the files its stamp lists,
// with matching digests, and nothing that is not a regular file or directory.
func verifyContent(root, version string) error {
	st, err := ReadContentStamp(root)
	if err != nil {
		return err
	}
	if st.Version != version {
		return fmt.Errorf("content is stamped %s, want %s", st.Version, version)
	}
	seen := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case rel == "." || rel == ContentStampName && d.Type().IsRegular():
			return nil
		case rel != "config" && !strings.HasPrefix(rel, "config/"):
			return fmt.Errorf("unexpected %s", rel)
		case d.IsDir():
			return nil
		case !d.Type().IsRegular():
			return fmt.Errorf("%s is not a regular file", rel)
		}
		want, ok := st.Files[rel]
		if !ok {
			return fmt.Errorf("%s is not in the content stamp", rel)
		}
		got, err := FileHash(p)
		if err != nil {
			return err
		}
		if got != want.SHA256 {
			return fmt.Errorf("%s does not match its content stamp", rel)
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(st.Files) {
		return fmt.Errorf("%d of %d stamped files are missing", len(st.Files)-seen, len(st.Files))
	}
	return nil
}

// sealContent makes a verified tree read-only: files take 0444 or 0555 from
// their stamp, directories 0555, deepest first so each is still writable while
// its children are changed.
func sealContent(root string) error {
	st, err := ReadContentStamp(root)
	if err != nil {
		return err
	}
	var dirs []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		mode := fs.FileMode(0o444)
		if st.Files[filepath.ToSlash(rel)].Mode == "0755" {
			mode = 0o555
		}
		return os.Chmod(p, mode)
	})
	if err != nil {
		return err
	}
	for _, d := range slices.Backward(dirs) {
		if err := os.Chmod(d, 0o555); err != nil {
			return err
		}
	}
	return nil
}

// removeStoreDir deletes a sealed tree, restoring owner write on its
// directories first since a read-only directory's entries cannot be removed.
func removeStoreDir(dir string) error {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	return RemoveManagedDir(dir)
}

// copyContentTree copies src's stamp and config/ tree into stage. Anything
// else at src's root is ignored; inside config/ only regular files and
// directories are accepted.
func copyContentTree(src, stage string) error {
	var files int
	var total int64
	if err := copyContentFile(filepath.Join(src, ContentStampName), filepath.Join(stage, ContentStampName), &files, &total); err != nil {
		return err
	}
	return filepath.WalkDir(filepath.Join(src, "config"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(stage, rel)
		switch {
		case d.IsDir():
			return os.Mkdir(out, 0o755)
		case d.Type().IsRegular():
			return copyContentFile(p, out, &files, &total)
		default:
			return fmt.Errorf("%s is not a regular file or directory", filepath.ToSlash(rel))
		}
	})
}

func copyContentFile(src, dst string, files *int, total *int64) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	if err := checkContentEntry(src, info.Mode(), info.Size(), files, total); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeContentFile(dst, in, info.Size())
}

// extractContentTarball writes a release content archive into stage. The
// archive wraps everything in one top directory, which is stripped. Paths
// must be clean and relative, entries must be regular files or directories,
// and only the stamp and config/ are written.
func extractContentTarball(archive, stage string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	var wrapper string
	var files int
	var total int64
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.Contains(name, `\`) ||
			slices.Contains(strings.Split(name, "/"), "..") {
			return fmt.Errorf("unsafe path %q", hdr.Name)
		}
		top, rel, _ := strings.Cut(name, "/")
		if wrapper == "" {
			wrapper = top
		} else if top != wrapper {
			return fmt.Errorf("%q is outside the archive's top directory %q", hdr.Name, wrapper)
		}
		switch hdr.Typeflag {
		case tar.TypeDir, tar.TypeReg:
		default:
			return fmt.Errorf("%s is not a regular file or directory", hdr.Name)
		}
		if rel == "" {
			if hdr.Typeflag != tar.TypeDir {
				return fmt.Errorf("%s: archive has no top directory", hdr.Name)
			}
			continue
		}
		if rel != ContentStampName && rel != "config" && !strings.HasPrefix(rel, "config/") {
			continue
		}
		out := filepath.Join(stage, filepath.FromSlash(rel))
		if hdr.Typeflag == tar.TypeDir {
			if rel == ContentStampName {
				return fmt.Errorf("%s is a directory", hdr.Name)
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := checkContentEntry(hdr.Name, hdr.FileInfo().Mode(), hdr.Size, &files, &total); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := writeContentFile(out, tr, hdr.Size); err != nil {
			return err
		}
	}
	if wrapper == "" {
		return errors.New("archive is empty")
	}
	return nil
}

func checkContentEntry(name string, mode fs.FileMode, size int64, files *int, total *int64) error {
	if mode&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
		return fmt.Errorf("%s has a setuid, setgid or sticky bit", name)
	}
	if size > maxContentFileSize {
		return fmt.Errorf("%s is %d bytes, over the %d limit", name, size, maxContentFileSize)
	}
	*files++
	*total += size
	if *files > maxContentFiles {
		return fmt.Errorf("more than %d files", maxContentFiles)
	}
	if *total > maxContentBytes {
		return fmt.Errorf("more than %d bytes", maxContentBytes)
	}
	return nil
}

// writeContentFile creates dst, refusing to overwrite (a duplicate archive
// entry is an error), and copies exactly size bytes.
func writeContentFile(dst string, r io.Reader, size int64) error {
	w, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(w, io.LimitReader(r, size))
	if err == nil && n != size {
		err = fmt.Errorf("%s: short read", dst)
	}
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return err
}

// validContentVersion accepts a version usable as a single directory name in
// the store, excluding the names the store uses for itself.
func validContentVersion(v string) error {
	if v == "" || v == "." || v == ".." || v == contentCurrentName || strings.HasSuffix(v, ".tmp") ||
		strings.ContainsAny(v, `/\`) || strings.HasPrefix(v, "-") {
		return fmt.Errorf("invalid content version %q", v)
	}
	return nil
}

// validContentPath accepts a clean, relative, slash-separated path under
// config/.
func validContentPath(rel string) bool {
	return (rel == "config" || strings.HasPrefix(rel, "config/")) && path.Clean(rel) == rel &&
		!strings.Contains(rel, `\`) && !slices.Contains(strings.Split(rel, "/"), "..")
}
