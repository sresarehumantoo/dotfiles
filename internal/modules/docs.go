package modules

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// DocsModule fetches offline usage documentation (the pages a project
// publishes on its website, not API dumps like pydoc) for config/devtools/docs
// to read. Most sets come from DevDocs, which serves hundreds of projects in
// one format; docTarballs covers projects it does not carry.
type DocsModule struct{}

func (DocsModule) Name() string { return "docs" }

// About 50 MB on disk together, from an 8 MB download.
var defaultDocSets = []string{"python", "bash", "git", "go", "docker", "kubernetes", "node"}

type docTarball struct {
	url string
	dir string // markdown tree inside the archive, below its top directory
}

var docTarballs = map[string]docTarball{
	// 42 MB, so it is opt-in through doc_sets rather than a default.
	"gitlab": {url: "https://gitlab.com/gitlab-org/gitlab/-/archive/master/gitlab-master.tar.gz?path=doc", dir: "doc"},
}

const (
	devdocsCatalogURL = "https://devdocs.io/docs.json"
	devdocsDocsURL    = "https://documents.devdocs.io"
	docSourceStamp    = ".source"
	docIndexFile      = "index.tsv"
)

// A set name becomes a directory and part of a URL, so it may not read as a
// path or an option.
var docSetNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._~-]*$`)

func docSets() []string {
	if core.Cfg.DocSets != nil {
		return core.Cfg.DocSets
	}
	return defaultDocSets
}

func docsDir(parts ...string) string { return core.SubPath(core.AppDataPath("docs"), parts...) }

type devdocsEntry struct {
	Slug    string `json:"slug"`
	Release string `json:"release"`
	Mtime   int64  `json:"mtime"`
}

// resolveDevdocs maps a set name to a catalog entry: an exact slug ("bash",
// "python~3.13") or, for a bare name, its newest version, which the catalog
// lists first.
func resolveDevdocs(catalog []devdocsEntry, name string) (devdocsEntry, bool) {
	for _, e := range catalog {
		if e.Slug == name {
			return e, true
		}
	}
	for _, e := range catalog {
		if strings.HasPrefix(e.Slug, name+"~") {
			return e, true
		}
	}
	return devdocsEntry{}, false
}

func readDocStamp(name string) string {
	if !docSetNameRe.MatchString(name) || strings.Contains(name, "..") {
		return ""
	}
	b, err := os.ReadFile(docsDir(name, docSourceStamp))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (m DocsModule) Install(ctx context.Context) error {
	sets := docSets()
	if core.DryRun {
		core.Info("would install w3m and fetch doc sets %v into %s", sets, docsDir())
		return nil
	}
	if err := installPkg(ctx, "w3m"); err != nil {
		core.Warn("w3m not installed (%v); DevDocs pages cannot be rendered without it", err)
	}

	var catalog []devdocsEntry
	var failed []string
	for _, name := range sets {
		if err := fetchDocSet(ctx, name, &catalog); err != nil {
			core.Warn("docs %s: %v", name, err)
			failed = append(failed, name)
		}
	}
	if err := pruneDocSets(sets); err != nil {
		return err
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d doc set(s) failed: %s", len(failed), strings.Join(failed, ", "))
	}
	core.Ok("docs done")
	return nil
}

// fetchDocSet leaves a set that is already present alone unless --upgrade
// asks for a refresh, and then refetches a DevDocs set only when its catalog
// entry changed. catalog is loaded on first need and shared across sets.
func fetchDocSet(ctx context.Context, name string, catalog *[]devdocsEntry) error {
	if !docSetNameRe.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid doc set name %q", name)
	}
	have := readDocStamp(name)
	if have != "" && !core.UpgradeMode {
		core.Ok("docs %s already present", name)
		return nil
	}

	if tb, ok := docTarballs[name]; ok {
		stamp := "tarball " + tb.url
		return stageDocSet(name, stamp, "source tarball", func(stage string) error { return fetchDocTarball(ctx, tb, stage) })
	}

	if *catalog == nil {
		data, err := fetchURL(ctx, devdocsCatalogURL)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, catalog); err != nil {
			return fmt.Errorf("parse DevDocs catalog: %w", err)
		}
	}
	e, ok := resolveDevdocs(*catalog, name)
	if !ok {
		return fmt.Errorf("no DevDocs set named %q (see https://devdocs.io/docs.json)", name)
	}
	stamp := fmt.Sprintf("devdocs %s %s %d", e.Slug, e.Release, e.Mtime)
	if stamp == have {
		core.Ok("docs %s up to date (%s)", name, e.Slug)
		return nil
	}
	return stageDocSet(name, stamp, e.Slug, func(stage string) error { return fetchDevdocs(ctx, e.Slug, stage) })
}

// pruneDocSets removes fetched sets no longer listed in doc_sets. Only
// directories carrying a source stamp are touched, so nothing dfinstall did
// not fetch is removed.
func pruneDocSets(keep []string) error {
	entries, err := os.ReadDir(docsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || slices.Contains(keep, name) || readDocStamp(name) == "" {
			continue
		}
		if err := core.RemoveManagedDir(docsDir(name)); err != nil {
			return err
		}
		core.Changed("removed docs %s (not in doc_sets)", name)
	}
	return nil
}

// stageDocSet builds a set beside its destination and swaps it in only once
// complete, so a failed fetch never costs the copy already there.
func stageDocSet(name, stamp, label string, fill func(stage string) error) error {
	dest := docsDir(name)
	if err := core.CheckTarget(dest); err != nil {
		return err
	}
	stage := dest + ".new"
	if err := core.RemoveManagedDir(stage); err != nil {
		return err
	}
	if err := core.EnsureDir(stage); err != nil {
		return err
	}
	defer func() { _ = core.RemoveManagedDir(stage) }()

	if err := fill(stage); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, docSourceStamp), []byte(stamp+"\n"), 0o644); err != nil {
		return err
	}
	if err := core.RemoveManagedDir(dest); err != nil {
		return err
	}
	if err := os.Rename(stage, dest); err != nil {
		return err
	}
	core.Changed("fetched docs %s (%s)", name, label)
	return nil
}

// fetchURL downloads through curl, like every other fetch here, so a corporate
// CA bundle in CURL_CA_BUNDLE is honored.
func fetchURL(ctx context.Context, url string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "dfinstall-docs-*")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := runNetCmd(ctx, "curl", curlArgs(url, "--compressed", "-o", tmp.Name())...); err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return os.ReadFile(tmp.Name())
}

// fetchDevdocs writes each page as <path>.html and an index of
// "name<TAB>path[#anchor]<TAB>type" lines, which is all the viewer reads.
func fetchDevdocs(ctx context.Context, slug, stage string) error {
	var index struct {
		Entries []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"entries"`
	}
	data, err := fetchURL(ctx, devdocsDocsURL+"/"+slug+"/index.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("parse %s index: %w", slug, err)
	}

	var db map[string]string
	if data, err = fetchURL(ctx, devdocsDocsURL+"/"+slug+"/db.json"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &db); err != nil {
		return fmt.Errorf("parse %s pages: %w", slug, err)
	}
	for p, html := range db {
		if err := writeDocPage(stage, p+".html", strings.NewReader(html)); err != nil {
			return err
		}
	}

	var b strings.Builder
	for _, e := range index.Entries {
		writeDocIndexLine(&b, e.Name, e.Path, e.Type)
	}
	return os.WriteFile(filepath.Join(stage, docIndexFile), []byte(b.String()), 0o644)
}

// fetchDocTarball extracts the markdown under tb.dir. A page is indexed by
// its path without ".md", and a section's index page (index.md, or Hugo's
// _index.md) by the section itself.
func fetchDocTarball(ctx context.Context, tb docTarball, stage string) error {
	tmp, err := os.CreateTemp("", "dfinstall-docs-*.tar.gz")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := runNetCmd(ctx, "curl", curlArgs(tb.url, "-o", tmp.Name())...); err != nil {
		return fmt.Errorf("download %s: %w", tb.url, err)
	}
	if err := extractDocTarball(tmp.Name(), tb.dir, stage); err != nil {
		return fmt.Errorf("%s: %w", tb.url, err)
	}
	return nil
}

func extractDocTarball(archive, dir, stage string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)

	var b strings.Builder
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || !strings.HasSuffix(h.Name, ".md") {
			continue
		}
		_, rest, ok := strings.Cut(h.Name, "/")
		if !ok {
			continue
		}
		rel, ok := strings.CutPrefix(rest, dir+"/")
		if !ok {
			continue
		}
		if err := writeDocPage(stage, rel, tr); err != nil {
			return err
		}
		name := strings.TrimSuffix(rel, ".md")
		if dir, base := path.Split(name); (base == "index" || base == "_index") && dir != "" {
			name = strings.TrimSuffix(dir, "/")
		}
		section, _, _ := strings.Cut(rel, "/")
		writeDocIndexLine(&b, name, rel, section)
	}
	if b.Len() == 0 {
		return fmt.Errorf("no markdown under %s/", dir)
	}
	return os.WriteFile(filepath.Join(stage, docIndexFile), []byte(b.String()), 0o644)
}

// writeDocPage refuses any page path that would land outside stage: both
// sources are remote, and their paths come straight from the download.
func writeDocPage(stage, rel string, r io.Reader) error {
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("refusing doc page path %q", rel)
	}
	dst := filepath.Join(stage, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeFile(dst, r)
}

var docIndexFieldCleaner = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func writeDocIndexLine(b *strings.Builder, fields ...string) {
	for i, f := range fields {
		if i > 0 {
			b.WriteByte('\t')
		}
		b.WriteString(docIndexFieldCleaner.Replace(f))
	}
	b.WriteByte('\n')
}

// Uninstall removes the fetched sets and leaves w3m, a general-purpose
// package the user may rely on elsewhere.
func (m DocsModule) Uninstall(ctx context.Context) error {
	dir := docsDir()
	if core.DryRun {
		core.Info("would remove %s", dir)
		return nil
	}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := core.RemoveManagedDir(dir); err != nil {
		return err
	}
	core.Changed("removed %s", dir)
	return nil
}

func (m DocsModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "docs"}
	var missing []string
	for _, name := range docSets() {
		if readDocStamp(name) != "" {
			s.Linked++
		} else {
			s.Missing++
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		s.Extra = "not fetched: " + strings.Join(missing, ", ")
	}
	return s
}
