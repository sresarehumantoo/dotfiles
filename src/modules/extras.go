package modules

import (
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

// addAptRepo sets up a third-party apt repository if not already configured.
func addAptRepo(name, keyURL, keyPath, repoContent, repoPath string) error {
	if existing, err := os.ReadFile(repoPath); err == nil {
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(repoContent) {
			core.Ok("%s repo already configured", name)
			return nil
		}
		core.Notice("Updating %s repo (content changed)", name)
	}

	core.Info("Adding %s apt repository...", name)

	// Download GPG key
	if err := runCmd("sudo", "mkdir", "-p", filepath.Dir(keyPath)); err != nil {
		return fmt.Errorf("creating keyring dir: %w", err)
	}
	dl := fmt.Sprintf("curl -fsSL %s | sudo tee %s > /dev/null", keyURL, keyPath)
	if err := runCmd("bash", "-c", dl); err != nil {
		return fmt.Errorf("downloading %s GPG key: %w", name, err)
	}

	// Write repo file (heredoc preserves newlines in DEB822 format)
	write := fmt.Sprintf("cat <<'REPO' | sudo tee %s > /dev/null\n%s\nREPO", repoPath, repoContent)
	if err := runCmd("bash", "-c", write); err != nil {
		return fmt.Errorf("writing %s repo file: %w", name, err)
	}

	// Update apt
	if err := runCmd("sudo", "apt-get", "update"); err != nil {
		return fmt.Errorf("apt-get update after adding %s repo: %w", name, err)
	}

	core.Ok("%s repo added", name)
	return nil
}

// dpkgInstalled checks if a Debian package is installed.
func dpkgInstalled(pkg string) bool {
	cmd := exec.Command("dpkg", "-s", pkg)
	return cmd.Run() == nil
}

// pacmanInstalled checks if a pacman package is installed.
func pacmanInstalled(pkg string) bool {
	return exec.Command("pacman", "-Qi", pkg).Run() == nil
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

func (ExtrasModule) Install() error {
	if core.DryRun {
		core.Info("would install: CLI utils, Python tooling, Docker, Terraform")
		return nil
	}

	// --- CLI utils ---
	core.Info("Installing CLI utilities...")
	cliPkgs := []string{
		"xclip", "tree", "fzf", "ripgrep", "fd-find",
		"bat", "jq", "unzip", "make", "build-essential", "tealdeer",
	}
	if err := installPkg(cliPkgs...); err != nil {
		core.Warn("Some CLI utils may have failed: %v", err)
	}

	// Update tldr page cache (best-effort — may fail on spotty networks)
	if _, err := exec.LookPath("tldr"); err == nil {
		core.Info("Updating tldr page cache...")
		if _, err := exec.Command("tldr", "--update").CombinedOutput(); err != nil {
			core.Info("tldr cache update skipped (network unavailable — run 'tldr --update' later)")
		}
	}

	core.Ok("CLI utilities done")

	// --- Python tooling ---
	core.Info("Installing Python tooling...")
	pythonPkgs := []string{"python3", "python3-pip", "python3-venv", "pipx"}
	if err := installPkg(pythonPkgs...); err != nil {
		core.Warn("Some Python packages may have failed: %v", err)
	}
	core.Ok("Python tooling done")

	// --- Docker ---
	core.Info("Installing Docker...")
	if err := installDocker(); err != nil {
		core.Warn("Docker setup failed: %v", err)
	} else {
		core.Ok("Docker done")
	}

	// --- Hashicorp / Terraform ---
	core.Info("Installing Terraform...")
	if err := installHashicorp(); err != nil {
		core.Warn("Terraform setup failed: %v", err)
	} else {
		core.Ok("Terraform done")
	}

	return nil
}

func installDocker() error {
	if core.IsArchBased() {
		return installDockerPacman()
	}
	return installDockerApt()
}

func installDockerApt() error {
	arch := runtime.GOARCH
	codename := distroCodename()

	repoContent := fmt.Sprintf(`Types: deb
URIs: https://download.docker.com/linux/debian
Suites: %s
Components: stable
Architectures: %s
Signed-By: /etc/apt/keyrings/docker.asc`, codename, arch)

	if err := addAptRepo(
		"Docker",
		"https://download.docker.com/linux/debian/gpg",
		"/etc/apt/keyrings/docker.asc",
		repoContent,
		"/etc/apt/sources.list.d/docker.sources",
	); err != nil {
		return err
	}

	pkgs := []string{
		"docker-ce", "docker-ce-cli", "containerd.io",
		"docker-buildx-plugin", "docker-compose-plugin",
	}
	if err := installPkg(pkgs...); err != nil {
		return fmt.Errorf("installing docker packages: %w", err)
	}

	addDockerGroup()
	return nil
}

func installDockerPacman() error {
	if err := installPkg("docker", "docker-compose", "docker-buildx"); err != nil {
		return fmt.Errorf("installing docker packages: %w", err)
	}

	addDockerGroup()
	return nil
}

func addDockerGroup() {
	if !userInGroup("docker") {
		u, err := user.Current()
		if err != nil {
			core.Warn("Failed to get current user: %v", err)
			return
		}
		core.Info("Adding %s to docker group...", u.Username)
		if err := runCmd("sudo", "usermod", "-aG", "docker", u.Username); err != nil {
			core.Warn("Failed to add user to docker group: %v", err)
		} else {
			core.Ok("Added %s to docker group (log out and back in to take effect)", u.Username)
		}
	}
}

func installHashicorp() error {
	if core.IsArchBased() {
		return installHashicorpBinary()
	}
	return installHashicorpApt()
}

func installHashicorpApt() error {
	arch := runtime.GOARCH

	repoContent := fmt.Sprintf(
		"deb [arch=%s signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com %s main",
		arch, distroCodename(),
	)

	if err := addAptRepo(
		"Hashicorp",
		"https://apt.releases.hashicorp.com/gpg",
		"/usr/share/keyrings/hashicorp-archive-keyring.gpg",
		repoContent,
		"/etc/apt/sources.list.d/hashicorp.list",
	); err != nil {
		return err
	}

	if err := installPkg("terraform"); err != nil {
		return fmt.Errorf("installing terraform: %w", err)
	}
	return nil
}

func installHashicorpBinary() error {
	if _, err := exec.LookPath("terraform"); err == nil {
		core.Ok("Terraform already installed")
		return nil
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}

	home, _ := os.UserHomeDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("creating bin dir: %w", err)
	}

	// Download latest terraform zip and extract to ~/.local/bin
	url := fmt.Sprintf("https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_%s.zip", arch)
	tmpZip := filepath.Join(os.TempDir(), "terraform.zip")
	if err := runCmd("curl", "-fsSL", "-o", tmpZip, url); err != nil {
		return fmt.Errorf("downloading terraform: %w", err)
	}
	defer os.Remove(tmpZip)

	if err := runCmd("unzip", "-o", tmpZip, "-d", binDir); err != nil {
		return fmt.Errorf("extracting terraform: %w", err)
	}

	core.Ok("Terraform installed to %s", binDir)
	return nil
}

// distroCodename reads VERSION_CODENAME from /etc/os-release.
func distroCodename() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "bookworm"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VERSION_CODENAME=") {
			return strings.TrimPrefix(line, "VERSION_CODENAME=")
		}
	}
	return "bookworm"
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
