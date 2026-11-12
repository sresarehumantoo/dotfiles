package modules

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type NewsboatModule struct{}

func (NewsboatModule) Name() string { return "newsboat" }

// newsboatPkgs: chafa renders images and w3m browses links in the terminal,
// both through config/devtools/newsboat-view.
var newsboatPkgs = []string{"newsboat", "chafa", "w3m"}

// newsboatLocalSeed is written only when config.local is absent. The tracked
// config includes it unconditionally, and newsboat refuses to start when an
// include is missing, so the file has to exist even when it holds nothing.
const newsboatLocalSeed = `# Machine-local newsboat settings, included last by ~/.config/newsboat/config.
# Not managed by dfinstall. To sync with a FreshRSS server:
#
#   urls-source "freshrss"
#   freshrss-url "https://freshrss.example.com/api/greader.php"
#   freshrss-login "me"
#   freshrss-passwordeval "pass show freshrss-api"
#   freshrss-min-items 100
#   freshrss-flag-star "s"
`

func newsboatLocal() string { return core.XDGTarget("newsboat", "config.local") }

func (NewsboatModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("newsboat", "config"), Dst: core.XDGTarget("newsboat", "config")},
		{Src: core.ConfigPath("newsboat", "urls"), Dst: core.XDGTarget("newsboat", "urls"), Policy: core.PolicySeed},
	}
}

func (m NewsboatModule) Install(ctx context.Context) error {
	if core.DryRun {
		core.Info("would install %v", newsboatPkgs)
	} else if err := installPkg(ctx, newsboatPkgs...); err != nil {
		core.Warn("newsboat packages not installed (%v); linking config anyway", err)
	}
	core.Info("Linking newsboat config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}
	if err := seedNewsboatLocal(); err != nil {
		return err
	}
	// newsboat (2.x) is XDG-first: it falls back to ~/.newsboat ONLY when the
	// XDG config is absent. We just linked ~/.config/newsboat/config above, so a
	// leftover ~/.newsboat no longer shadows it -- measured on 2.36: with the
	// XDG config present, newsboat ignores ~/.newsboat entirely, even when that
	// dir holds its own config+urls. Warning on mere existence fired on every
	// run and was false, so warn only in the case newsboat would actually use
	// the legacy dir: the XDG config is not in place (e.g. the link failed).
	if legacy := core.HomeTarget(".newsboat"); legacy != "" {
		_, legacyErr := os.Stat(legacy)
		_, xdgErr := os.Stat(core.XDGTarget("newsboat", "config"))
		if legacyErr == nil && errors.Is(xdgErr, fs.ErrNotExist) {
			core.AlwaysWarn("%s exists and ~/.config/newsboat/config is missing, so newsboat will use the legacy dir; move its cache.db to ~/.local/share/newsboat and remove it", legacy)
		}
	}
	core.Ok("newsboat config done")
	return nil
}

func seedNewsboatLocal() error {
	path := newsboatLocal()
	if err := core.CheckTarget(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err := core.WriteFileIfChanged(path, []byte(newsboatLocalSeed), 0o644)
	return err
}

// Uninstall leaves config.local in place: it is the user's file, and may hold
// credentials wiring they would not want deleted along with the links.
func (m NewsboatModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("newsboat config uninstalled")
	return nil
}

func (m NewsboatModule) Status() core.ModuleStatus { return m.Links().Status("newsboat") }
