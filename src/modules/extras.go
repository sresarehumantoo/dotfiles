package modules

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type ExtrasModule struct{}

func (ExtrasModule) Name() string { return "extras" }

// writeFileAsRoot writes data to a root-owned file via a tmp-then-install
// dance. Replaces the `cat <<EOF | sudo tee` pattern so the sudo invocation
// goes through the proper sudo handling (spinner pause, stderr connected).
func writeFileAsRoot(ctx context.Context, dst string, data []byte, mode os.FileMode) error {
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
			core.Ok("%s repo already configured", name)
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

	core.Ok("%s repo added", name)
	return nil
}

// dpkgInstalled checks if a Debian package is installed.
// dpkgInstalled and pacmanInstalled are called from Status() as well as the
// install path, so they have no context to inherit. runProbe/CommandContext
// still bound them with ProbeTimeout, which is what stops a wedged dpkg from
// hanging the run.
func dpkgInstalled(pkg string) bool {
	cmd := exec.CommandContext(context.Background(), "dpkg", "-s", pkg)
	return cmd.Run() == nil
}

// pacmanInstalled checks if a pacman package is installed.
func pacmanInstalled(pkg string) bool {
	return exec.CommandContext(context.Background(), "pacman", "-Qi", pkg).Run() == nil
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
	return dpkgInstalled(pkg)
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
		core.Info("would install: CLI utils, Python tooling, Docker, Terraform")
		return nil
	}

	// --- CLI utils ---
	core.Info("Installing CLI utilities...")
	cliWanted := []struct {
		bin  string
		pkgs []string
	}{
		{"xclip", []string{"xclip"}},
		{"tree", []string{"tree"}},
		{"fzf", []string{"fzf"}},
		{"rg", []string{"ripgrep"}},
		{"fdfind", []string{"fd-find"}},
		{"batcat", []string{"bat"}},
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
		if _, err := exec.CommandContext(ctx, "tldr", "--update").CombinedOutput(); err != nil {
			core.Info("tldr cache update skipped (network unavailable — run 'tldr --update' later)")
		}
	}

	core.Ok("CLI utilities done")

	// --- Python tooling ---
	core.Info("Installing Python tooling...")
	var pythonPkgs []string
	for _, pkg := range []string{"python3-pip", "python3-venv", "pipx"} {
		if !dpkgInstalled(pkg) {
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
	if core.IsArchBased() {
		return installDockerPacman(ctx)
	}
	return installDockerApt(ctx)
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

	addDockerGroup(ctx)
	return nil
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

	addDockerGroup(ctx)
	return nil
}

func addDockerGroup(ctx context.Context) {
	if !userInGroup("docker") {
		u, err := user.Current()
		if err != nil {
			core.Warn("Failed to get current user: %v", err)
			return
		}
		core.Info("Adding %s to docker group...", u.Username)
		if err := runCmd(ctx, "sudo", "usermod", "-aG", "docker", u.Username); err != nil {
			core.Warn("Failed to add user to docker group: %v", err)
		} else {
			core.Ok("Added %s to docker group (log out and back in to take effect)", u.Username)
		}
	}
}

func installHashicorp(ctx context.Context) error {
	if core.IsArchBased() {
		return installHashicorpBinary(ctx)
	}
	return installHashicorpApt(ctx)
}

func installHashicorpApt(ctx context.Context) error {
	arch := runtime.GOARCH

	// The /gpg endpoint serves an ASCII-armored key, so it must land at a .asc
	// path (apt treats a .gpg keyring as binary/dearmored and fails to verify
	// it — "NO_PUBKEY ... not signed"). Mirrors the working Docker key handling.
	repoContent := fmt.Sprintf(
		"deb [arch=%s signed-by=/usr/share/keyrings/hashicorp-archive-keyring.asc] https://apt.releases.hashicorp.com %s main",
		arch, core.UpstreamDebianCodename(),
	)

	if err := addAptRepo(ctx,
		"Hashicorp",
		"https://apt.releases.hashicorp.com/gpg",
		"/usr/share/keyrings/hashicorp-archive-keyring.asc",
		repoContent,
		"/etc/apt/sources.list.d/hashicorp.list",
	); err != nil {
		return err
	}

	if err := installPkg(ctx, "terraform"); err != nil {
		return fmt.Errorf("installing terraform: %w", err)
	}
	return nil
}

// terraformVersion is the release fetched by installHashicorpBinary. HashiCorp
// publishes no "latest" alias on releases.hashicorp.com, so this is pinned and
// must be bumped by hand.
const terraformVersion = "1.9.8"

func installHashicorpBinary(ctx context.Context) error {
	if _, err := exec.LookPath("terraform"); err == nil {
		core.Ok("Terraform already installed")
		return nil
	}

	// Only these two are published for linux; anything else would build a URL
	// that 404s with no explanation.
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("no terraform binary published for %s", arch)
	}

	binDir := core.HomeTarget(".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("creating bin dir: %w", err)
	}

	// Download the pinned terraform zip and extract to ~/.local/bin
	url := fmt.Sprintf("https://releases.hashicorp.com/terraform/%[1]s/terraform_%[1]s_linux_%[2]s.zip", terraformVersion, arch)
	tmpZip := filepath.Join(os.TempDir(), "terraform.zip")
	if err := runCmd(ctx, "curl", "-fsSL", "-o", tmpZip, url); err != nil {
		return fmt.Errorf("downloading terraform: %w", err)
	}
	defer os.Remove(tmpZip)

	if err := runCmd(ctx, "unzip", "-o", tmpZip, "-d", binDir); err != nil {
		return fmt.Errorf("extracting terraform: %w", err)
	}

	core.Ok("Terraform installed to %s", binDir)
	return nil
}

func (ExtrasModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "extras"}

	// CLI utils — binary names differ by distro
	fdBin := "fdfind"
	batBin := "batcat"
	if core.IsArchBased() {
		fdBin = "fd"
		batBin = "bat"
	}

	cliChecks := []struct {
		binary string
		pkg    bool // check via package manager instead of binary
	}{
		{"xclip", false},
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

	// Docker (2 checks: binary + group)
	if _, err := exec.LookPath("docker"); err == nil {
		s.Linked++
	} else {
		s.Missing++
	}
	if userInGroup("docker") {
		s.Linked++
	} else {
		s.Missing++
	}

	// Hashicorp (1 check)
	if _, err := exec.LookPath("terraform"); err == nil {
		s.Linked++
	} else {
		s.Missing++
	}

	return s
}
