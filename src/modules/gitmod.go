package modules

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type GitModule struct{}

func (GitModule) Name() string { return "git" }

func (GitModule) Install() error {
	core.Info("Linking git config...")
	if err := core.LinkFile(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")); err != nil {
		return err
	}

	localCfg := core.HomeTarget(".gitconfig.local")
	if _, err := os.Stat(localCfg); os.IsNotExist(err) {
		if core.DryRun {
			core.Info("[dry-run] Would prompt for git identity and write %s", localCfg)
		} else {
			core.PauseSpinner()
			if err := promptGitIdentity(localCfg); err != nil {
				core.ResumeSpinner()
				core.Warn("Could not set git identity — run: git config --global user.name/email")
			} else {
				core.ResumeSpinner()
			}
		}
	}

	core.Ok("Git config done")
	return nil
}

func promptGitIdentity(path string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("  Git user.name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)

	fmt.Print("  Git user.email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)

	if name == "" || email == "" {
		core.Warn("Skipping git identity — name or email was empty")
		return nil
	}

	content := fmt.Sprintf("[user]\n\tname = %s\n\temail = %s\n", name, email)
	return os.WriteFile(path, []byte(content), 0644)
}

func (GitModule) Uninstall() error {
	if err := core.UnlinkFile(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")); err != nil {
		return err
	}
	core.Ok("Git config uninstalled")
	return nil
}

func (GitModule) Links() []core.LinkPair {
	return []core.LinkPair{
		{Src: core.ConfigPath("git", "gitconfig"), Dst: core.HomeTarget(".gitconfig")},
	}
}

func (GitModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "git"}
	if core.CheckLink(core.ConfigPath("git", "gitconfig"), core.HomeTarget(".gitconfig")) == "ok" {
		s.Linked++
	} else {
		s.Missing++
	}
	return s
}
