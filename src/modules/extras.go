package modules

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type ExtrasModule struct{}

func (ExtrasModule) Name() string { return "extras" }

// writeFileAsRoot writes data to a root-owned file via a tmp-then-install
// dance. Replaces the `cat <<EOF | sudo tee` pattern so the sudo invocation
// goes through the proper sudo handling (spinner pause, stderr connected).
// A file that already holds data with that mode is left alone, so a re-run
// does not ask for a password.
func writeFileAsRoot(ctx context.Context, dst string, data []byte, mode os.FileMode) error {
	if fi, err := os.Stat(dst); err == nil && fi.Mode().Perm() == mode.Perm() {
		if cur, err := os.ReadFile(dst); err == nil && bytes.Equal(cur, data) {
			return nil
		}
	}
	tmp, err := os.CreateTemp("", "dfinstall-asroot-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	modeStr := fmt.Sprintf("%#o", mode)
	if err := runCmd(ctx, "sudo", "install", "-m", modeStr, "-o", "root", "-g", "root", tmpPath, dst); err != nil {
		return fmt.Errorf("install %s: %w", dst, err)
	}
	return nil
}

// downloadAndInstallAsRoot fetches a URL to a temp file (as the user) then
// installs it to a root-owned destination. Replaces the `curl | sudo tee`
// pattern.
func downloadAndInstallAsRoot(ctx context.Context, url, dst string, mode os.FileMode) error {
	tmp, err := os.CreateTemp("", "dfinstall-dl-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := runCmd(ctx, "curl", "-fsSL", "-o", tmpPath, url); err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}

	modeStr := fmt.Sprintf("%#o", mode)
	if err := runCmd(ctx, "sudo", "install", "-m", modeStr, "-o", "root", "-g", "root", tmpPath, dst); err != nil {
		return fmt.Errorf("install %s: %w", dst, err)
	}
	return nil
}

// addAptRepo sets up a third-party apt repository if not already configured.
func addAptRepo(ctx context.Context, name, keyURL, keyPath, repoContent, repoPath string) error {
	if existing, err := os.ReadFile(repoPath); err == nil {
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(repoContent) {
			// The repo line being unchanged says nothing about the signing
			// key. Upstreams rotate keys on their own schedule, and a stale
			// key makes apt refuse the index while this function, the repo
			// file, and the installed binary all still look fine — so the
			// key has to be checked separately or it is never checked again
			// after the day the repo was first added.
			changed, err := ensureAptRepoKey(ctx, name, keyURL, keyPath, repoContent)
			if err != nil {
				core.Warn("%s signing key check: %v", name, err)
			}
			if !changed {
				core.Ok("%s repo already configured", name)
				return nil
			}
			if err := aptUpdateWithRetry(ctx); err != nil {
				return fmt.Errorf("apt update after refreshing %s key: %w", name, err)
			}
			core.Changed("%s signing key refreshed", name)
			return nil
		}
		core.Notice("Updating %s repo (content changed)", name)
	}

	core.Info("Adding %s apt repository...", name)

	// Download GPG key
	if err := runCmd(ctx, "sudo", "mkdir", "-p", filepath.Dir(keyPath)); err != nil {
		return fmt.Errorf("creating keyring dir: %w", err)
	}
	if err := downloadAndInstallAsRoot(ctx, keyURL, keyPath, 0644); err != nil {
		return fmt.Errorf("downloading %s GPG key: %w", name, err)
	}

	// Write repo file (heredoc preserves newlines in DEB822 format)
	if err := writeFileAsRoot(ctx, repoPath, []byte(repoContent), 0644); err != nil {
		return fmt.Errorf("writing %s repo file: %w", name, err)
	}

	// Update apt
	if err := aptUpdateWithRetry(ctx); err != nil {
		return fmt.Errorf("apt update after adding %s repo: %w", name, err)
	}

	core.Changed("%s repo added", name)
	return nil
}

// dpkgInstalled checks if a Debian package is installed. Not `dpkg -s`, which
// also succeeds for a package removed with its config files left behind
// (dpkg -l state "rc").
// dpkgInstalled and pacmanInstalled are called from Status() as well as the
// install path, so they have no context to inherit. runProbe bounds them with
// ProbeTimeout, which is what stops a wedged dpkg from hanging the run.
func dpkgInstalled(pkg string) bool {
	out, err := runProbe(context.Background(), "dpkg-query", "-W", "-f=${db:Status-Status}", pkg)
	return err == nil && string(out) == "installed"
}

// pacmanInstalled checks if a pacman package is installed.
func pacmanInstalled(pkg string) bool {
	_, err := runProbe(context.Background(), "pacman", "-Qi", pkg)
	return err == nil
}

// pkgInstalled checks if a package is installed using the appropriate package manager.
func pkgInstalled(pkg string) bool {
	if core.IsArchBased() {
		resolved := resolvePkg("pacman", pkg)
		if resolved == "" {
			return true // not needed on Arch
		}
		return pacmanInstalled(resolved)
	}
	if core.IsMac() {
		resolved := resolvePkg("brew", pkg)
		if resolved == "" {
			return true // not needed on macOS
		}
		return brewInstalled(resolved)
	}
	return dpkgInstalled(pkg)
}

// brewInstalled checks if a Homebrew formula is installed.
func brewInstalled(pkg string) bool {
	_, err := runProbe(context.Background(), "brew", "list", "--versions", pkg)
	return err == nil
}

// fdBatBins returns the binary names fd and bat install under. Debian renames
// both to dodge older packages that already claimed the names.
func fdBatBins() (fd, bat string) {
	if core.IsArchBased() || core.IsMac() {
		return "fd", "bat"
	}
	return "fdfind", "batcat"
}

// userInGroup checks if the current user belongs to the given group.
func userInGroup(group string) bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return false
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true
		}
	}
	return false
}

func (ExtrasModule) Install(ctx context.Context) error {
	if core.DryRun {
		core.Info("would install: CLI utils, Python tooling, Rust, Docker, Terraform")
		return nil
	}

	// --- CLI utils ---
	core.Info("Installing CLI utilities...")
	fdBin, batBin := fdBatBins()
	cliWanted := []struct {
		bin  string
		pkgs []string
	}{
		{"xclip", []string{"xclip"}},
		// nvim, the clip alias and tmux-yank all prefer wl-copy under
		// Wayland (sway, WSLg); xclip only works while XWayland is up.
		{"wl-copy", []string{"wl-clipboard"}},
		{"tree", []string{"tree"}},
		{"fzf", []string{"fzf"}},
		{"rg", []string{"ripgrep"}},
		{fdBin, []string{"fd-find"}},
		{batBin, []string{"bat"}},
		{"jq", []string{"jq"}},
		{"unzip", []string{"unzip"}},
		{"make", []string{"make"}},
		{"gcc", []string{"build-essential"}},
		{"tldr", []string{"tealdeer"}},
	}
	var cliPkgs []string
	for _, w := range cliWanted {
		if _, err := exec.LookPath(w.bin); err != nil {
			cliPkgs = append(cliPkgs, w.pkgs...)
		}
	}
	if len(cliPkgs) == 0 {
		core.Ok("All CLI utilities already installed")
	} else {
		if err := installPkg(ctx, cliPkgs...); err != nil {
			core.Warn("Some CLI utils may have failed: %v", err)
		}
	}

	// Update tldr page cache (best-effort — may fail on spotty networks)
	if _, err := exec.LookPath("tldr"); err == nil {
		core.Info("Updating tldr page cache...")
		if _, err := runNetProbe(ctx, "tldr", "--update"); err != nil {
			core.Info("tldr cache update skipped (network unavailable; run 'tldr --update' later)")
		}
	}

	core.Ok("CLI utilities done")

	// --- Python tooling ---
	core.Info("Installing Python tooling...")
	var pythonPkgs []string
	for _, pkg := range []string{"python3-pip", "python3-venv", "pipx"} {
		if !pkgInstalled(pkg) {
			pythonPkgs = append(pythonPkgs, pkg)
		}
	}
	if len(pythonPkgs) == 0 {
		core.Ok("Python tooling already installed")
	} else {
		if err := installPkg(ctx, pythonPkgs...); err != nil {
			core.Warn("Some Python packages may have failed: %v", err)
		}
		core.Ok("Python tooling done")
	}

	// --- Rust ---
	// rust-analyzer (Mason) needs a toolchain and the std sources, which only
	// rustup provides; distro and brew rust ship neither rust-src nor a way
	// to add it.
	core.Info("Installing Rust...")
	if installRustup(ctx) {
		if err := runCmd(ctx, rustupExe(), "component", "add", "rust-src"); err != nil {
			core.Warn("rustup component add rust-src failed: %v", err)
		} else {
			core.Ok("Rust done")
		}
	}

	// --- Docker ---
	core.Info("Installing Docker...")
	if err := installDocker(ctx); err != nil {
		core.Warn("Docker setup failed: %v", err)
	} else {
		core.Ok("Docker done")
	}

	// --- Hashicorp / Terraform ---
	core.Info("Installing Terraform...")
	if err := installHashicorp(ctx); err != nil {
		core.Warn("Terraform setup failed: %v", err)
	} else {
		core.Ok("Terraform done")
	}

	return nil
}

func installDocker(ctx context.Context) error {
	if core.IsMac() {
		return installDockerColima(ctx)
	}
	if core.IsArchBased() {
		return installDockerPacman(ctx)
	}
	return installDockerApt(ctx)
}

// installDockerColima installs the Docker CLI with Colima as its runtime:
// Docker Engine only runs on Linux, and Docker Desktop carries license terms.
func installDockerColima(ctx context.Context) error {
	var missing []string
	for _, f := range []string{"colima", "docker", "docker-compose", "docker-buildx"} {
		if !brewInstalled(f) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		if err := installPkg(ctx, missing...); err != nil {
			return err
		}
	}

	brew, err := exec.LookPath("brew")
	if err != nil {
		return err
	}
	pluginDir := filepath.Join(filepath.Dir(filepath.Dir(brew)), "lib", "docker", "cli-plugins")
	if err := addDockerPluginDir(core.HomeTarget(".docker", "config.json"), pluginDir); err != nil {
		return fmt.Errorf("registering compose/buildx with docker: %w", err)
	}

	if _, err := runProbe(ctx, "colima", "status"); err != nil {
		core.Notice("Docker runs in Colima: start it with `colima start`, or `brew services start colima` to start it at login")
	}
	return nil
}

// addDockerPluginDir lists dir in cliPluginsExtraDirs of the docker CLI config
// at cfgPath. Homebrew installs compose and buildx as CLI plugins under its own
// prefix, which docker never searches otherwise, so `docker compose` would not
// exist. The rest of the file (registry auths, contexts) is kept as it is, and
// a file that does not parse is left untouched.
func addDockerPluginDir(cfgPath, dir string) error {
	if err := core.CheckTarget(cfgPath); err != nil {
		return err
	}
	cfg := map[string]any{}
	data, err := os.ReadFile(cfgPath)
	switch {
	case err == nil:
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&cfg); err != nil {
			return fmt.Errorf("%s: %w", cfgPath, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	var dirs []any
	if v, ok := cfg["cliPluginsExtraDirs"]; ok {
		if dirs, ok = v.([]any); !ok {
			return fmt.Errorf("%s: cliPluginsExtraDirs is not a list", cfgPath)
		}
	}
	for _, d := range dirs {
		if d == dir {
			return nil
		}
	}
	cfg["cliPluginsExtraDirs"] = append(dirs, dir)

	out, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}
	// 0700/0600: the file can hold registry credentials.
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(cfgPath), ".config.json-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(out, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), cfgPath)
}

// DockerRepoBaseURL returns the Docker CE apt repo base for this distro family.
// Docker ships separate trees: Ubuntu(-family) hosts must use linux/ubuntu,
// while Debian and its derivatives use linux/debian. Pairing linux/debian with
// an Ubuntu codename (e.g. "noble") points at a suite that doesn't exist.
func DockerRepoBaseURL(ubuntuFamily bool) string {
	if ubuntuFamily {
		return "https://download.docker.com/linux/ubuntu"
	}
	return "https://download.docker.com/linux/debian"
}

func installDockerApt(ctx context.Context) error {
	arch := runtime.GOARCH
	codename := core.UpstreamDebianCodename()
	base := DockerRepoBaseURL(core.IsUbuntuFamily())

	repoContent := fmt.Sprintf(`Types: deb
URIs: %s
Suites: %s
Components: stable
Architectures: %s
Signed-By: /etc/apt/keyrings/docker.asc`, base, codename, arch)

	if err := writeFileAsRoot(ctx, "/etc/apt/preferences.d/docker", []byte(aptOriginPin("download.docker.com")), 0644); err != nil {
		return fmt.Errorf("writing apt pin: %w", err)
	}
	if err := addAptRepo(ctx,
		"Docker",
		base+"/gpg",
		"/etc/apt/keyrings/docker.asc",
		repoContent,
		"/etc/apt/sources.list.d/docker.sources",
	); err != nil {
		return err
	}

	// Remove distro-shipped Docker packages before installing Docker CE.
	// Some distros (Parrot, Kali, Mint LMDE) preinstall the Debian-packaged
	// docker.io / docker-compose. They own files like
	// /usr/libexec/docker/cli-plugins/docker-compose that the official
	// docker-compose-plugin package also wants to install — dpkg refuses
	// to overwrite, the install fails, and the apt cache is left dirty.
	// Docker's own install docs require this removal step.
	removeConflictingDockerPackages(ctx)

	pkgs := []string{
		"docker-ce", "docker-ce-cli", "containerd.io",
		"docker-buildx-plugin", "docker-compose-plugin",
	}
	if err := installPkg(ctx, pkgs...); err != nil {
		return fmt.Errorf("installing docker packages: %w", err)
	}

	setupRootlessDocker(ctx)
	return nil
}

// rootlessDockerUnit is the user unit dockerd-rootless-setuptool.sh writes.
func rootlessDockerUnit() string {
	return core.SubPath(core.XDGConfigHome(), "systemd", "user", "docker.service")
}

// setupRootlessDocker runs the daemon as the user instead of adding them to
// the docker group, whose members can mount / into a container as root. The
// system daemon stays installed for what rootless cannot do (--privileged
// work on the host kernel, such as registering binfmt_misc handlers), via
// `sudo docker`. Someone already in the group keeps the setup they have:
// switching the CLI context would hide their existing images and containers.
func setupRootlessDocker(ctx context.Context) {
	if userInGroup("docker") {
		core.Info("Already in the docker group (root-equivalent); rootless Docker not set up")
		return
	}
	unit := rootlessDockerUnit()
	if unit == "" {
		return
	}
	if _, err := os.Stat(unit); err == nil {
		core.Ok("Rootless Docker already set up")
		return
	}
	// uidmap (setuid newuidmap/newgidmap) and slirp4netns are not among
	// rootless-extras' dependencies, and only this path needs them.
	if err := installPkg(ctx, "docker-ce-rootless-extras", "uidmap", "slirp4netns"); err != nil {
		core.Warn("Rootless Docker packages: %v", err)
		return
	}
	const manual = "dockerd-rootless-setuptool.sh install"
	if _, err := runProbe(ctx, "systemctl", "--user", "show-environment"); err != nil {
		core.Notice("Rootless Docker needs a systemd user session: run `%s` after logging in, or use `sudo docker`", manual)
		return
	}
	if _, err := runProbe(ctx, "rootlesskit", "true"); err != nil {
		core.Warn("Rootless Docker is unavailable here (rootlesskit failed; see https://rootlesscontaine.rs/getting-started/common/). Use `sudo docker`")
		return
	}
	core.Info("Setting up rootless Docker...")
	if err := runCmd(ctx, "dockerd-rootless-setuptool.sh", "install"); err != nil {
		core.Warn("Rootless Docker setup failed: %v (retry with `%s`, or use `sudo docker`)", err, manual)
		return
	}
	core.Changed("Rootless Docker set up; `sudo docker` reaches the system daemon")
}

// removeConflictingDockerPackages purges distro-shipped Docker packages
// that overlap with Docker CE. Per Docker's official install instructions
// for Debian/Ubuntu derivatives.
func removeConflictingDockerPackages(ctx context.Context) {
	candidates := []string{
		"docker.io",
		"docker-compose",
		"docker-doc",
		"docker-compose-v2",
		"podman-docker",
		"containerd",
		"runc",
	}
	var present []string
	for _, p := range candidates {
		if dpkgInstalled(p) {
			present = append(present, p)
		}
	}
	if len(present) == 0 {
		return
	}
	core.Notice("Removing distro packages that conflict with Docker CE: %v", present)
	args := append([]string{"sudo", "apt-get", "remove", "-y"}, present...)
	if err := runCmd(ctx, args[0], args[1:]...); err != nil {
		core.Warn("conflict removal: %v (will try install anyway)", err)
	}
}

func installDockerPacman(ctx context.Context) error {
	if err := installPkg(ctx, "docker", "docker-compose", "docker-buildx"); err != nil {
		return fmt.Errorf("installing docker packages: %w", err)
	}

	// Arch's docker package has no rootless mode (that is the AUR's
	// docker-rootless-extras), and the docker group is root-equivalent.
	if !userInGroup("docker") {
		core.Info("Docker installed; use `sudo docker`, or set up rootless mode from the AUR's docker-rootless-extras")
	}
	return nil
}

func installHashicorp(ctx context.Context) error {
	dst := core.HomeTarget(".local", "bin", "terraform")
	upgrade := !core.IsMac() && upgradeable(dst)
	if localBinInstalled("terraform") && !upgrade {
		core.Ok("Terraform already installed")
		return nil
	}
	if core.IsMac() {
		return installPkg(ctx, "hashicorp/tap/terraform")
	}
	rel, err := latestTerraform(ctx)
	if err != nil {
		if upgrade {
			core.Warn("Terraform upgrade check: %v", err)
			return nil
		}
		return err
	}
	if upgrade && releaseCurrent(ctx, dst, rel.Version, "version") {
		core.Ok("Terraform %s is the latest release", rel.Version)
		return nil
	}
	return installTerraformRelease(ctx, rel)
}

// hashicorpKeyFingerprint is HashiCorp's release-signing key (primary key).
// The key itself is fetched at install time; this pin is what makes it
// HashiCorp's rather than whatever the download returned.
const hashicorpKeyFingerprint = "C874011F0AB405110D02105534365D9472D7468F"

// terraformRelease is the part of releases.hashicorp.com's release JSON that
// installTerraformRelease reads.
type terraformRelease struct {
	Version string           `json:"version"`
	Builds  []terraformBuild `json:"builds"`
	SHASums string           `json:"url_shasums"`
}

type terraformBuild struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	URL  string `json:"url"`
}

// latestTerraform fetches the latest release's metadata from HashiCorp.
func latestTerraform(ctx context.Context) (terraformRelease, error) {
	var rel terraformRelease
	out, err := runNetProbe(ctx, "curl", "-fsSL", "https://api.releases.hashicorp.com/v1/releases/terraform/latest")
	if err != nil {
		return rel, fmt.Errorf("fetch terraform release: %w", err)
	}
	if err := json.Unmarshal(out, &rel); err != nil {
		return rel, fmt.Errorf("parse terraform release: %w", err)
	}
	return rel, nil
}

// installTerraformRelease installs a terraform release binary into
// ~/.local/bin, checked against HashiCorp's signed SHA256SUMS. This replaces
// adding HashiCorp's apt repository, which needed root and let that repo
// supply any package to apt, for the sake of one static binary.
func installTerraformRelease(ctx context.Context, rel terraformRelease) error {
	dst := core.HomeTarget(".local", "bin", "terraform")
	if err := core.CheckTarget(dst); err != nil {
		return err
	}
	i := slices.IndexFunc(rel.Builds, func(b terraformBuild) bool {
		return b.OS == "linux" && b.Arch == runtime.GOARCH
	})
	if i < 0 || rel.SHASums == "" {
		return fmt.Errorf("no terraform %s build for linux/%s", rel.Version, runtime.GOARCH)
	}
	zipURL := rel.Builds[i].URL

	tmp, err := os.MkdirTemp("", "terraform-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	zipPath := filepath.Join(tmp, path.Base(zipURL))
	sums := filepath.Join(tmp, "SHA256SUMS")
	for _, f := range []struct{ url, dst string }{{zipURL, zipPath}, {rel.SHASums, sums}} {
		if err := runNetCmd(ctx, "curl", "-fsSL", "-o", f.dst, f.url); err != nil {
			return fmt.Errorf("download %s: %w", path.Base(f.url), err)
		}
	}

	if err := verifyHashicorpSignature(ctx, tmp, sums, rel.SHASums); err != nil {
		return err
	}
	listing, err := os.ReadFile(sums)
	if err != nil {
		return err
	}
	want, ok := parseSHALine(string(listing), path.Base(zipURL))
	if !ok {
		return fmt.Errorf("no entry for %s in SHA256SUMS", path.Base(zipURL))
	}
	got, err := core.FileHash(zipPath)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("sha256 mismatch for %s: want %s, got %s", path.Base(zipURL), want, got)
	}

	bin := filepath.Join(tmp, "terraform")
	if err := extractZipFile(zipPath, "terraform", bin); err != nil {
		return err
	}
	if err := runCmd(ctx, "install", "-D", "-m", "0755", bin, dst); err != nil {
		return err
	}
	core.Changed("Terraform %s installed to %s", rel.Version, dst)
	return nil
}

// verifyHashicorpSignature checks SHA256SUMS against its detached signature
// and requires the signer to be the pinned HashiCorp key. Without gpg it says
// so and goes on with the checksum alone, which still catches a corrupt or
// swapped download, just not a compromised release host.
func verifyHashicorpSignature(ctx context.Context, tmp, sums, sumsURL string) error {
	if _, err := exec.LookPath("gpg"); err != nil {
		core.AlwaysWarn("gpg not found: terraform is checked against SHA256SUMS, but its signature is not verified")
		return nil
	}
	home := filepath.Join(tmp, "gnupg")
	if err := os.Mkdir(home, 0o700); err != nil {
		return err
	}
	key := filepath.Join(tmp, "hashicorp.asc")
	sig := sums + ".sig"
	keyID := hashicorpKeyFingerprint[len(hashicorpKeyFingerprint)-8:]
	for _, f := range []struct{ url, dst string }{
		{"https://www.hashicorp.com/.well-known/pgp-key.txt", key},
		{sumsURL + "." + keyID + ".sig", sig},
	} {
		if err := runNetCmd(ctx, "curl", "-fsSL", "-o", f.dst, f.url); err != nil {
			return fmt.Errorf("download %s: %w", path.Base(f.url), err)
		}
	}
	if _, err := runProbe(ctx, "gpg", "--batch", "--homedir", home, "--import", key); err != nil {
		return fmt.Errorf("import HashiCorp key: %w", err)
	}
	// gpg exits non-zero on a bad signature, but a good signature from some
	// other key in the file would still pass; VALIDSIG's last field is the
	// signer's primary key, which is what the pin names.
	status, err := runProbe(ctx, "gpg", "--batch", "--homedir", home, "--status-fd", "1", "--verify", sig, sums)
	if err != nil {
		return fmt.Errorf("terraform SHA256SUMS signature did not verify: %w", err)
	}
	if !validSigFrom(string(status), hashicorpKeyFingerprint) {
		return fmt.Errorf("terraform SHA256SUMS is not signed by HashiCorp's key %s", hashicorpKeyFingerprint)
	}
	return nil
}

// validSigFrom reports whether gpg --status-fd output holds a VALIDSIG whose
// primary-key fingerprint (its last field) is fpr.
func validSigFrom(status, fpr string) bool {
	for _, line := range strings.Split(status, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "[GNUPG:]" && f[1] == "VALIDSIG" && strings.EqualFold(f[len(f)-1], fpr) {
			return true
		}
	}
	return false
}

// extractZipFile writes the archive member named name to dst.
func extractZipFile(zipPath, name, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != name {
			continue
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("%s has no %s", filepath.Base(zipPath), name)
}

func (ExtrasModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "extras"}

	fdBin, batBin := fdBatBins()
	clipBin := "xclip"
	if core.IsMac() {
		clipBin = "pbcopy"
	}

	cliChecks := []struct {
		binary string
		pkg    bool // check via package manager instead of binary
	}{
		{clipBin, false},
		{"tree", false},
		{"fzf", false},
		{"rg", false},
		{fdBin, false},
		{batBin, false},
		{"jq", false},
		{"unzip", false},
		{"make", false},
		{"build-essential", true},
		{"tldr", false},
	}
	for _, c := range cliChecks {
		if c.pkg {
			if pkgInstalled(c.binary) {
				s.Linked++
			} else {
				s.Missing++
			}
		} else {
			if _, err := exec.LookPath(c.binary); err == nil {
				s.Linked++
			} else {
				s.Missing++
			}
		}
	}

	// Python tooling
	pythonBins := []string{"python3", "pip3", "pipx"}
	for _, b := range pythonBins {
		if _, err := exec.LookPath(b); err == nil {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	if pkgInstalled("python3-venv") {
		s.Linked++
	} else {
		s.Missing++
	}

	// Docker (2 checks: binary + a way to reach a daemon without sudo, either
	// rootless mode or an existing docker group membership; Colima on macOS)
	if _, err := exec.LookPath("docker"); err == nil {
		s.Linked++
	} else {
		s.Missing++
	}
	if core.IsMac() {
		if _, err := exec.LookPath("colima"); err == nil {
			s.Linked++
		} else {
			s.Missing++
		}
	} else if _, err := os.Stat(rootlessDockerUnit()); err == nil || userInGroup("docker") {
		s.Linked++
	} else {
		s.Missing++
	}

	// Hashicorp (1 check)
	if localBinInstalled("terraform") {
		s.Linked++
	} else {
		s.Missing++
	}

	// Signing keys for the apt repos added above (1 check each). A present
	// binary says nothing about whether the repo it came from can still be
	// updated, which is how a dead keyring stays invisible. Arch hosts get
	// these tools from binaries or the AUR, and macOS from Homebrew, so neither
	// has a keyring to check.
	//
	// This is deliberately offline, so it catches an absent, corrupt, or
	// expired keyring but not a rotation — see aptKeyringUsable.
	if !core.IsArchBased() && !core.IsMac() {
		for _, keyPath := range []string{
			"/etc/apt/keyrings/docker.asc",
		} {
			if aptKeyringUsable(keyPath) {
				s.Linked++
			} else {
				s.Missing++
			}
		}
	}

	return s
}
