package modules

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

type GitModule struct{}

func (GitModule) Name() string { return "git" }

func (m GitModule) Install(ctx context.Context) error {
	core.Info("Linking git config...")
	if err := m.Links().Apply(); err != nil {
		return err
	}

	// Only a missing identity is worth a question. The file it would go in is
	// often more than an identity (site credential helpers, url rewrites, or a
	// symlink into another repo), so it is added to, never rewritten.
	localCfg := core.HomeTarget(".gitconfig.local")
	missing := missingGitIdentity(ctx)
	switch {
	case len(missing) == 0:
		core.Info("git identity already set")
	case core.DryRun:
		core.Info("[dry-run] Would prompt for %s and add it to %s", strings.Join(missing, ", "), localCfg)
	case !core.Interactive:
		// Reading stdin here would consume the caller's protocol stream.
		core.Notice("git %s not set; set it with: git config --file ~/.gitconfig.local %s <value>", strings.Join(missing, ", "), missing[0])
	default:
		core.PauseSpinner()
		err := promptGitIdentity(ctx, localCfg, missing)
		core.ResumeSpinner()
		if err != nil {
			core.Warn("Could not set git identity: %v. Run: git config --file ~/.gitconfig.local user.email <value>", err)
		}
	}

	core.Ok("Git config done")
	return nil
}

// missingGitIdentity lists the identity keys no global config sets, reading
// ~/.gitconfig with its includes, so ~/.gitconfig.local counts.
func missingGitIdentity(ctx context.Context) []string {
	var missing []string
	for _, key := range []string{"user.name", "user.email"} {
		out, err := runProbe(ctx, "git", "config", "--global", "--includes", "--get", key)
		if err != nil || strings.TrimSpace(string(out)) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

// promptGitIdentity asks for the missing keys and adds them to path with git
// config, which leaves the rest of the file, and a symlink to it, intact.
func promptGitIdentity(ctx context.Context, path string, missing []string) error {
	_, statErr := os.Stat(path)
	fresh := os.IsNotExist(statErr)
	reader := bufio.NewReader(os.Stdin)

	for _, key := range missing {
		fmt.Printf("  Git %s: ", key)
		value, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if value = strings.TrimSpace(value); value == "" {
			core.Warn("Skipping %s: no value given", key)
			continue
		}
		if err := runCmd(ctx, "git", "config", "--file", path, key, value); err != nil {
			return err
		}
	}

	// Only on first setup: asked on every run, it was answered no forever.
	if !fresh {
		return nil
	}
	fmt.Print("  Use git credential store? (for HTTPS auth e.g. GitLab) [y/N]: ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	if a := strings.TrimSpace(strings.ToLower(answer)); a == "y" || a == "yes" {
		return runCmd(ctx, "git", "config", "--file", path, "credential.helper", "store")
	}
	return nil
}

func (m GitModule) Uninstall(ctx context.Context) error {
	if err := m.Links().Remove(); err != nil {
		return err
	}
	core.Ok("Git config uninstalled")
	return nil
}

func (GitModule) Links() core.LinkSet {
	return core.LinkSet{
		{Src: core.ConfigPath("git", "gitconfig"), Dst: core.HomeTarget(".gitconfig")},
	}
}

func (m GitModule) Status() core.ModuleStatus { return m.Links().Status("git") }
