package modules

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type LocaleModule struct{}

func (LocaleModule) Name() string { return "locale" }

// localeGenerated checks if the given locale is available on the system.
func localeGenerated(name string) bool {
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		return false
	}
	// locale -a outputs names like "en_US.utf8" (no hyphen, lowercase)
	target := strings.ReplaceAll(strings.ToLower(name), "-", "")
	for _, line := range strings.Split(string(out), "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == target {
			return true
		}
	}
	return false
}

func (LocaleModule) Install() error {
	if core.DryRun {
		core.Info("would configure locale: en_US.UTF-8")
		return nil
	}

	if localeGenerated("en_US.UTF-8") {
		core.Ok("Locale en_US.UTF-8 already available")
		return nil
	}

	core.Info("Configuring locale...")

	// Install locales package if locale-gen is missing
	if _, err := exec.LookPath("locale-gen"); err != nil {
		if err := installPkg("locales"); err != nil {
			core.Warn("Failed to install locales package: %v", err)
			return nil
		}
	}

	// Uncomment en_US.UTF-8 in /etc/locale.gen (or append if absent)
	genPath := "/etc/locale.gen"
	data, err := os.ReadFile(genPath)
	if err != nil {
		core.Warn("Cannot read %s: %v", genPath, err)
		return nil
	}

	content := string(data)
	if strings.Contains(content, "# en_US.UTF-8 UTF-8") {
		// Uncomment the existing line
		if err := runCmd("sudo", "sed", "-i", "s/^# *en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/", genPath); err != nil {
			core.Warn("Failed to uncomment locale in %s: %v", genPath, err)
			return nil
		}
	} else if !strings.Contains(content, "en_US.UTF-8 UTF-8") {
		// Not present at all — append
		if err := runCmd("bash", "-c", "echo 'en_US.UTF-8 UTF-8' | sudo tee -a "+genPath+" > /dev/null"); err != nil {
			core.Warn("Failed to append locale to %s: %v", genPath, err)
			return nil
		}
	}

	// Generate the locale
	if err := runCmd("sudo", "locale-gen"); err != nil {
		core.Warn("locale-gen failed: %v", err)
		return nil
	}

	// Set as system default
	if err := runCmd("sudo", "update-locale", "LANG=en_US.UTF-8"); err != nil {
		core.Warn("update-locale failed: %v", err)
		return nil
	}

	core.Ok("Locale en_US.UTF-8 configured")
	return nil
}

func (LocaleModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "locale"}

	if _, err := exec.LookPath("locale-gen"); err == nil {
		s.Linked++
	} else {
		s.Missing++
	}

	if localeGenerated("en_US.UTF-8") {
		s.Linked++
	} else {
		s.Missing++
	}

	return s
}
