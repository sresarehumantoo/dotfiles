package modules

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type ExtrasModule struct{}

func (ExtrasModule) Name() string { return "extras" }

// addAptRepo sets up a third-party apt repository if not already configured.
func addAptRepo(name, keyURL, keyPath, repoContent, repoPath string) error {
	if _, err := os.Stat(repoPath); err == nil {
		core.Ok("%s repo already configured", name)
		return nil
	}

	core.Info("Adding %s apt repository...", name)

	// Download GPG key
	if err := runCmd("sudo", "mkdir", "-p", fmt.Sprintf("%s", dir(keyPath))); err != nil {
		return fmt.Errorf("creating keyring dir: %w", err)
	}
	dl := fmt.Sprintf("curl -fsSL %s | sudo tee %s > /dev/null", keyURL, keyPath)
	if err := runCmd("bash", "-c", dl); err != nil {
		return fmt.Errorf("downloading %s GPG key: %w", name, err)
	}

	// Write repo file
	write := fmt.Sprintf("echo %q | sudo tee %s > /dev/null", repoContent, repoPath)
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

// dir returns the directory portion of a path.
func dir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

// dpkgInstalled checks if a Debian package is installed.
func dpkgInstalled(pkg string) bool {
	cmd := exec.Command("dpkg", "-s", pkg)
	return cmd.Run() == nil
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
	// --- CLI utils ---
	core.Info("Installing CLI utilities...")
	cliPkgs := []string{
		"xclip", "tree", "fzf", "ripgrep", "fd-find",
		"bat", "jq", "unzip", "make", "build-essential",
	}
	if err := installPkg(cliPkgs...); err != nil {
		core.Warn("Some CLI utils may have failed: %v", err)
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
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}

	// Detect distro codename
	codename := "bookworm" // default for Debian
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERSION_CODENAME=") {
				codename = strings.TrimPrefix(line, "VERSION_CODENAME=")
				break
			}
		}
	}

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

	// Add current user to docker group if not already a member
	if !userInGroup("docker") {
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("getting current user: %w", err)
		}
		core.Info("Adding %s to docker group...", u.Username)
		if err := runCmd("sudo", "usermod", "-aG", "docker", u.Username); err != nil {
			core.Warn("Failed to add user to docker group: %v", err)
		} else {
			core.Ok("Added %s to docker group (log out and back in to take effect)", u.Username)
		}
	}

	return nil
}

func installHashicorp() error {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}

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

	// CLI utils (10 checks)
	cliChecks := []struct {
		binary string
		dpkg   bool
	}{
		{"xclip", false},
		{"tree", false},
		{"fzf", false},
		{"rg", false},     // ripgrep binary name
		{"fdfind", false}, // fd-find binary name on Debian
		{"batcat", false}, // bat binary name on Debian
		{"jq", false},
		{"unzip", false},
		{"make", false},
		{"build-essential", true}, // check via dpkg
	}
	for _, c := range cliChecks {
		if c.dpkg {
			if dpkgInstalled(c.binary) {
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

	// Python tooling (4 checks)
	pythonBins := []string{"python3", "pip3", "pipx"}
	for _, b := range pythonBins {
		if _, err := exec.LookPath(b); err == nil {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	if dpkgInstalled("python3-venv") {
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
