package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

// OpencodeModule installs opencode set up for Anthropic API key and Bedrock
// users: only those two providers load, Zen's free models and session sharing
// are off, and permissions follow Claude Code's default mode.
//
// It is opt-in like windev: excluded from `install all` until `install
// opencode` sets core.Cfg.OpencodeEnabled.
//
// It targets opencode 2. GitHub's "latest" release and npm's opencode-ai
// still point at the 1.x line, so the release is picked by tag and the npm
// package is @opencode/cli.
//
// The binary comes from the GitHub release or from npm, whichever is likelier
// to work here: a machine whose npm points at a mirror (Nexus, Artifactory)
// tries npm first, since a corporate proxy may block GitHub downloads. Either
// source falls back to the other. Both publish the same bytes.
type OpencodeModule struct{}

func (OpencodeModule) Name() string { return "opencode" }

const opencodeRepo = "anomalyco/opencode"

// Links reuses the Claude Code instructions as opencode's global rules.
// opencode 2 reads only AGENTS.md, never ~/.claude/CLAUDE.md.
func (OpencodeModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("claude", "CLAUDE.md"), Dst: core.XDGTarget("opencode", "AGENTS.md")},
	}
}

func opencodeSettings() managedJSON {
	return managedJSON{
		label: "opencode settings",
		base:  core.ConfigPath("opencode", "opencode.jsonc"),
		live:  core.XDGTarget("opencode", "opencode.json"),
	}
}

// opencodeOverrides returns the config files present beside opencode.json that
// opencode also loads. It reads opencode.json and then opencode.jsonc, the
// later winning, so a stray opencode.jsonc silently undoes every managed key.
//
// A file holding nothing but $schema is skipped: opencode writes exactly that
// opencode.jsonc on a first run with no config, so anyone who tried opencode
// before installing this module has one.
func opencodeOverrides() []string {
	var found []string
	for _, name := range []string{"opencode.jsonc"} {
		p := core.XDGTarget("opencode", name)
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if m, err := readJSONCObject(p); err == nil {
			if _, ok := m["$schema"]; ok && len(m) == 1 {
				continue
			}
		}
		found = append(found, p)
	}
	return found
}

func opencodeEnvPath() string { return core.AppConfigPath("opencode.zsh") }

// opencodeLocal is where dfinstall puts the binary, from either source.
func opencodeLocal() string { return core.HomeTarget(".local", "bin", "opencode") }

// opencodeNpmDir is the npm prefix the npm source installs into.
func opencodeNpmDir() string { return core.AppDataPath("opencode") }

func (m OpencodeModule) Install(ctx context.Context) error {
	core.Info("Setting up opencode...")
	if o := opencodeOverrides(); len(o) > 0 {
		return fmt.Errorf("%s would override the managed settings in opencode.json; move what you need into opencode.json and remove it",
			strings.Join(o, " and "))
	}
	if err := m.Links().Apply(); err != nil {
		return err
	}
	if err := opencodeSettings().merge(); err != nil {
		return err
	}
	if err := writeOpencodeEnv(); err != nil {
		return err
	}
	if err := ensureOpencode(ctx); err != nil {
		return err
	}
	return pinOpencodeService(ctx)
}

// opencodePinned are the managed keys re-applied through
// OPENCODE_CONFIG_CONTENT, which opencode loads after a project's
// opencode.json and .opencode/. Without it a cloned repo can turn sharing back
// on and loosen permissions: a project's permission rules are appended after
// the user's, and the last match wins. The provider allow-list and the plugin
// deny need no pin, since for policies the user's own file wins over a
// project's. A project can still give one of its agents its own permissions,
// which are applied after these.
var opencodePinned = []string{"enabled_providers", "disabled_providers", "share", "plugin", "autoupdate", "permission"}

// opencodePin returns the pinned keys as the JSON OPENCODE_CONFIG_CONTENT takes.
func opencodePin() (string, error) {
	base, err := readJSONCObject(opencodeSettings().base)
	if err != nil {
		return "", err
	}
	pin := map[string]any{}
	for _, k := range opencodePinned {
		if v, ok := base[k]; ok {
			pin[k] = v
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(pin); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

// opencodeServiceEnv is the environment both the shell and the background
// service get.
func opencodeServiceEnv() (map[string]string, error) {
	pin, err := opencodePin()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"OPENCODE_CONFIG_CONTENT":      pin,
		"OPENCODE_DISABLE_FREE_MODELS": "1",
		"OPENCODE_DISABLE_AUTOUPDATE":  "1",
	}, nil
}

// writeOpencodeEnv writes the zsh snippet zshrc sources: the pinned
// environment, a stricter alias for untrusted checkouts, and the wrapper that
// starts the TUI in mini mode.
func writeOpencodeEnv() error {
	env, err := opencodeServiceEnv()
	if err != nil {
		return err
	}
	content := `# Generated by dfinstall (opencode module). Regenerated on each
# 'dfinstall install opencode'; edits will be overwritten.
export OPENCODE_CONFIG_CONTENT=` + zshQuote(env["OPENCODE_CONFIG_CONTENT"]) + `
export OPENCODE_DISABLE_FREE_MODELS=1
export OPENCODE_DISABLE_AUTOUPDATE=1
# For a checkout you don't trust: ignore its opencode.json, .opencode/ and
# AGENTS.md. --standalone because the shared background service reads its
# environment once, when it starts, and would not see this variable.
alias opencode-untrusted='OPENCODE_DISABLE_PROJECT_CONFIG=1 opencode --standalone'

# The TUI starts as 'opencode mini', which writes to the terminal's own
# scrollback, so tmux copy-mode and the mouse wheel work. By default mini
# clears the scrollback (ESC[3J) and reprints the session on every resize,
# which in tmux wipes the pane's history on each zoom or split; --no-replay
# stops that. A resume keeps replay, or the earlier conversation is not shown
# at all. Inside tmux, opencode-tmux runs it so tmux-resurrect can bring the
# conversation back. Subcommands and a directory argument pass through, and
# 'command opencode' starts the full-screen TUI.
opencode() {
  if [[ $# -gt 0 && $1 != -* ]]; then
    command opencode "$@"
    return
  fi
  local replay=--no-replay a
  for a in "$@"; do
    case $a in
      -c|--continue|-s|--session|--session=*) replay= ;;
    esac
  done
  if [[ -n $TMUX ]] && (( $+commands[opencode-tmux] )); then
    opencode-tmux run mini $replay "$@"
  else
    command opencode mini $replay "$@"
  fi
}
`
	_, err = core.WriteFileIfChanged(opencodeEnvPath(), []byte(content), 0o644)
	return err
}

func opencodeServiceConfig() string { return core.XDGTarget("opencode", "service.json") }

// opencodeServiceStale returns the pinned variables missing from, or different
// in, the background service's own environment, and whether its service.json
// exists at all.
func opencodeServiceStale() (stale []string, exists bool) {
	want, err := opencodeServiceEnv()
	if err != nil {
		return nil, false
	}
	var have struct {
		Env map[string]string `json:"env"`
	}
	data, err := os.ReadFile(opencodeServiceConfig())
	if err == nil {
		exists = true
		_ = json.Unmarshal(data, &have)
	}
	for k, v := range want {
		if have.Env[k] != v {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	return stale, exists
}

// opencodeServiceDrift is what status and diff report. Without a service.json
// there is nothing to compare: opencode 2 creates it on first use, and install
// writes the pins into it.
func opencodeServiceDrift() []string {
	if stale, exists := opencodeServiceStale(); exists {
		return stale
	}
	return nil
}

// pinOpencodeService gives the background service the same environment as the
// shell. opencode 2 TUIs share one service, which reads its environment once,
// from whichever shell happened to start it; the env in its own service.json
// is applied over that, so the pins hold whoever starts it. Writes go through
// 'opencode service set' because that file also holds the service password.
func pinOpencodeService(ctx context.Context) error {
	bin := opencodeBinary()
	if bin == "" || opencodeMajor(ctx, bin) < 2 {
		return nil
	}
	stale, _ := opencodeServiceStale()
	if len(stale) == 0 {
		core.Ok("opencode service environment already pinned")
		return nil
	}
	if core.DryRun {
		core.Changed("would set %s in the opencode service config", strings.Join(stale, ", "))
		return nil
	}
	env, err := opencodeServiceEnv()
	if err != nil {
		return err
	}
	for _, k := range stale {
		if err := runCmd(ctx, bin, "service", "set", "env", k, env[k]); err != nil {
			return fmt.Errorf("opencode service set env %s: %w", k, err)
		}
	}
	core.Changed("set %s in the opencode service config", strings.Join(stale, ", "))
	if out, err := runProbe(ctx, bin, "service", "status"); err == nil && !strings.Contains(string(out), "stopped") {
		core.Warn("the opencode service is running with its old environment; restart it with: opencode service restart")
	}
	return nil
}

// opencodeBinary is the opencode this module configures: the one it
// installed, else whichever is on PATH.
func opencodeBinary() string {
	if local := opencodeLocal(); local != "" {
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	p, _ := exec.LookPath("opencode")
	return p
}

// opencodeMajor is the major version bin reports, or 0 when it will not say.
func opencodeMajor(ctx context.Context, bin string) int {
	out, err := runProbe(ctx, bin, "--version")
	if err != nil {
		return 0
	}
	v := semverRe.FindString(string(out))
	major, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	return major
}

// zshQuote single-quotes s for a shell.
func zshQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ensureOpencode installs the binary, or upgrades the one dfinstall installed.
// An opencode installed some other way is left to whoever installed it.
func ensureOpencode(ctx context.Context) error {
	local := opencodeLocal()
	if err := core.CheckTarget(local); err != nil {
		return err
	}
	_, err := os.Lstat(local)
	ours := err == nil
	if !ours {
		if p, err := exec.LookPath("opencode"); err == nil {
			if opencodeMajor(ctx, p) < 2 {
				core.Warn("%s is opencode 1, which this setup no longer targets; remove it and run: dfinstall install opencode", p)
			} else {
				core.Ok("using opencode at %s (not installed by dfinstall)", p)
			}
			return nil
		}
	} else if !core.UpgradeMode && opencodeMajor(ctx, local) >= 2 {
		core.Ok("opencode already installed")
		return nil
	}

	target, ok := opencodeTargetFor(runtime.GOOS, runtime.GOARCH, hasAVX2(ctx))
	if !ok {
		return fmt.Errorf("no opencode build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	sources := opencodeSources(ctx)
	if core.DryRun {
		verb := "install"
		if ours {
			verb = "update"
		}
		core.Info("would %s opencode (%s) from %s", verb, target, sources[0].name)
		return nil
	}

	var errs []error
	for _, src := range sources {
		err := src.install(ctx, target, local)
		if err == nil {
			return nil
		}
		core.Warn("opencode from %s: %v", src.name, err)
		errs = append(errs, fmt.Errorf("%s: %w", src.name, err))
	}
	if ours {
		core.Warn("keeping the installed opencode")
		return nil
	}
	return fmt.Errorf("could not install opencode: %w", errors.Join(errs...))
}

type opencodeSource struct {
	name    string
	install func(ctx context.Context, target, dst string) error
}

// opencodeSources orders the places to get opencode from: npm first when it
// is pointed at a mirror, else the GitHub release, with the other as fallback.
// npm is offered only when it is installed.
func opencodeSources(ctx context.Context) []opencodeSource {
	gh := opencodeSource{"the GitHub release", installOpencodeRelease}
	reg := npmRegistry(ctx)
	if reg == "" {
		return []opencodeSource{gh}
	}
	npm := opencodeSource{"npm (" + redactURL(reg) + ")", installOpencodeNpm}
	if npmMirrored(reg) {
		return []opencodeSource{npm, gh}
	}
	return []opencodeSource{gh, npm}
}

// opencodeTargetFor names opencode's build for a platform, as used in both
// release asset and npm package names. The baseline x64 build is for CPUs
// without AVX2. musl builds are never picked: no supported distro needs them.
func opencodeTargetFor(goos, goarch string, avx2 bool) (string, bool) {
	if goos != "linux" && goos != "darwin" {
		return "", false
	}
	switch goarch {
	case "arm64":
		return goos + "-arm64", true
	case "amd64":
		if avx2 {
			return goos + "-x64", true
		}
		return goos + "-x64-baseline", true
	}
	return "", false
}

func hasAVX2(ctx context.Context) bool {
	if runtime.GOARCH != "amd64" {
		return false
	}
	if runtime.GOOS == "darwin" {
		out, err := runProbe(ctx, "sysctl", "-n", "hw.optional.avx2_0")
		return err == nil && strings.TrimSpace(string(out)) == "1"
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "flags") {
			return strings.Contains(line+" ", " avx2 ")
		}
	}
	return false
}

// opencodeAssetName is the release asset for target.
func opencodeAssetName(target string) string {
	if strings.HasPrefix(target, "darwin-") {
		return "opencode-" + target + ".zip"
	}
	return "opencode-" + target + ".tar.gz"
}

// opencodeV2Tag matches the opencode 2 release tags, which share the
// repository with 1.x releases and the process-daemon's.
var opencodeV2Tag = regexp.MustCompile(`^v2\.(\d+)\.(\d+)$`)

// pickOpencodeRelease returns the newest opencode 2 release in a GitHub
// release listing. The listing is ordered by creation, and a 1.x patch can be
// created after a 2.x release, so the tags are compared rather than taken in
// order.
func pickOpencodeRelease(listing []byte) (string, []ghAsset, error) {
	var releases []struct {
		Tag        string    `json:"tag_name"`
		Draft      bool      `json:"draft"`
		Prerelease bool      `json:"prerelease"`
		Assets     []ghAsset `json:"assets"`
	}
	if err := json.Unmarshal(listing, &releases); err != nil {
		return "", nil, fmt.Errorf("parse opencode releases: %w", err)
	}
	best, bestMinor, bestPatch := -1, -1, -1
	for i, r := range releases {
		m := opencodeV2Tag.FindStringSubmatch(r.Tag)
		if m == nil || r.Draft || r.Prerelease {
			continue
		}
		minor, _ := strconv.Atoi(m[1])
		patch, _ := strconv.Atoi(m[2])
		if minor > bestMinor || (minor == bestMinor && patch > bestPatch) {
			best, bestMinor, bestPatch = i, minor, patch
		}
	}
	if best < 0 {
		return "", nil, fmt.Errorf("no opencode 2 release in the latest %d", len(releases))
	}
	return releases[best].Tag, releases[best].Assets, nil
}

func installOpencodeRelease(ctx context.Context, target, dst string) error {
	listing, err := githubProbe(ctx, "https://api.github.com/repos/"+opencodeRepo+"/releases?per_page=50")
	if err != nil {
		return fmt.Errorf("fetch releases for %s: %w", opencodeRepo, err)
	}
	tag, assets, err := pickOpencodeRelease(listing)
	if err != nil {
		return err
	}
	if releaseCurrent(ctx, dst, tag) {
		core.Ok("opencode %s is the latest release", tag)
		return nil
	}
	name := opencodeAssetName(target)
	var asset ghAsset
	for _, a := range assets {
		if a.Name == name {
			asset = a
		}
	}
	if asset.Name == "" {
		return fmt.Errorf("release %s has no %s", tag, name)
	}

	tmp, err := os.MkdirTemp("", "opencode-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, name)
	if err := downloadVerified(ctx, asset, archive); err != nil {
		return err
	}
	if strings.HasSuffix(name, ".zip") {
		err = runCmd(ctx, "unzip", "-q", archive, "-d", tmp)
	} else {
		err = runCmd(ctx, "tar", "-xzf", archive, "-C", tmp)
	}
	if err != nil {
		return err
	}
	if err := placeFile(filepath.Join(tmp, "opencode"), dst, 0o755); err != nil {
		return err
	}
	core.Changed("installed opencode %s from %s", tag, name)
	return nil
}

// installOpencodeNpm installs the platform package (@opencode/cli-linux-x64
// and so on) rather than @opencode/cli, a stub whose postinstall script picks
// the binary out of that same package; going straight to it means no install
// script runs at all.
func installOpencodeNpm(ctx context.Context, target, dst string) error {
	pkg := "@opencode/cli-" + target
	// One retry, not npm's default backoff: a registry that is down here
	// should hand over to the GitHub release in seconds, not over a minute.
	out, err := runNetProbe(ctx, "npm", "view", "--fetch-retries=1", pkg, "version")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(out))
	if !semverRe.MatchString(version) || strings.ContainsAny(version, " /@") {
		return fmt.Errorf("unexpected version %q for %s", version, pkg)
	}
	if releaseCurrent(ctx, dst, version) {
		core.Ok("opencode %s is the latest on npm", version)
		return nil
	}

	dir := opencodeNpmDir()
	if err := core.CheckTarget(dir); err != nil {
		return err
	}
	if err := runNetCmd(ctx, "npm", "install", "--prefix", dir,
		"--ignore-scripts", "--no-audit", "--no-fund", "--no-save", "--no-package-lock",
		pkg+"@"+version); err != nil {
		return err
	}
	bin := filepath.Join(dir, "node_modules", filepath.FromSlash(pkg), "bin", "opencode")
	if !releaseCurrent(ctx, bin, version) {
		return fmt.Errorf("%s does not report version %s", bin, version)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Symlink(bin, dst); err != nil {
		return err
	}
	core.Changed("installed opencode %s from npm (%s)", version, pkg)
	return nil
}

// placeFile copies src to dst through a temp file and a rename, so dst is
// replaced rather than written through: it may be a symlink into the npm
// prefix, or a binary that is running.
func placeFile(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
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
	return os.Rename(tmp.Name(), dst)
}

// Uninstall removes what the module put down. The merged settings stay, since
// taking the keys out would turn free models and sharing back on, and so do
// opencode's credentials and sessions.
func (m OpencodeModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	paths := []string{opencodeEnvPath(), opencodeLocal()}
	if core.DryRun {
		core.Info("would remove %s, %s and %s", paths[0], paths[1], opencodeNpmDir())
		return nil
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := os.Remove(p); err == nil {
			core.Changed("removed %s", p)
		} else if !errors.Is(err, os.ErrNotExist) {
			core.Warn("could not remove %s: %v", p, err)
		}
	}
	if dir := opencodeNpmDir(); dir != "" {
		if _, err := os.Stat(dir); err == nil {
			if err := core.RemoveManagedDir(dir); err != nil {
				return err
			}
			core.Changed("removed %s", dir)
		}
	}
	core.Ok("opencode uninstalled (opencode.json and credentials left as is)")
	return nil
}

// Status counts only the link, so it agrees with Links(); the binary and the
// settings are reported in Extra.
func (m OpencodeModule) Status() core.ModuleStatus {
	if !core.Cfg.OpencodeEnabled {
		return core.ModuleStatus{Name: "opencode", Extra: "disabled"}
	}
	s := m.Links().Status("opencode")
	var notes []string
	if !localBinInstalled("opencode") {
		notes = append(notes, "opencode not installed")
	}
	for _, p := range opencodeOverrides() {
		notes = append(notes, filepath.Base(p)+" overrides opencode.json")
	}
	if e := opencodeSettings().statusExtra(); e != "" {
		notes = append(notes, e)
	}
	if stale := opencodeServiceDrift(); len(stale) > 0 {
		notes = append(notes, "service environment not pinned")
	}
	s.Extra = strings.Join(notes, "; ")
	return s
}

// ContentDrift reports managed keys changed in opencode.json, and config files
// that override it.
func (OpencodeModule) ContentDrift() []DiffProblem {
	d := opencodeSettings().drift()
	for _, p := range opencodeOverrides() {
		d = append(d, DiffProblem{Dst: p, State: "content", Detail: "loaded after opencode.json, overrides managed keys"})
	}
	if stale := opencodeServiceDrift(); len(stale) > 0 {
		d = append(d, DiffProblem{Dst: opencodeServiceConfig(), State: "content", Detail: "service env differs: " + strings.Join(stale, ", ")})
	}
	return d
}
