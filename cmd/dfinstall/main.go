package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
	"github.com/sresarehumantoo/dotfiles/v2/internal/modules"
)

var (
	flagVerbose  bool
	flagDebug    bool
	flagDryRun   bool
	flagBackup   bool
	flagExtended bool
	flagToolkit  bool
	flagUpgrade  bool
	flagBrief    bool
	flagRegistry string
)

// upgradedEnv marks a dfinstall re-executed after --upgrade rebuilt it, so the
// new binary skips the pull it has just done.
const upgradedEnv = "DFINSTALL_UPGRADED"

func main() {
	modules.RegisterAllModules()

	rootCmd := &cobra.Command{
		Use:     "dfinstall",
		Short:   "Dotfiles installer and manager",
		Version: core.Version + " (" + core.BuildChannel() + ")",
		// Runtime failures (a module erroring, a backup that won't start) are
		// not usage errors — printing the full help after them buries the
		// actual message. Cobra still shows usage for bad arguments.
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			switch {
			case flagDebug:
				core.Level = core.LogDebug
			case flagVerbose:
				core.Level = core.LogVerbose
			}
			if flagDryRun {
				core.DryRun = true
				if core.Level < core.LogVerbose {
					core.Level = core.LogVerbose
				}
			}
			// A config that exists but can't be read is not a first run.
			// Warn loudly and carry on with defaults — read-only commands
			// still work, and SaveConfig refuses to overwrite it.
			if err := core.LoadConfig(); err != nil {
				core.AlwaysWarn("%v", err)
				core.AlwaysWarn("continuing with defaults; settings will not be saved until this is fixed")
			}
			// --brief output is read by the shell's drift check, not a person.
			if !(cmd.Name() == "diff" && flagBrief) {
				core.PrintBanner()
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Show debug output (implies verbose)")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without modifying the filesystem")

	runInstall := func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if os.Geteuid() == 0 {
			core.Err("Running as root is not supported.")
			core.Err("Run as your normal user instead. To apply configs to /root, use:")
			core.Err("    dfinstall root")
			return fmt.Errorf("refusing to install as root")
		}

		core.DetectEnvironment()
		core.AssertEnvironment()
		if err := core.RequireDotfiles(); err != nil {
			return err
		}

		// Infer module from standalone flags when no positional arg given
		var target string
		switch {
		case len(args) > 0:
			target = args[0]
		case flagToolkit:
			target = "toolkit"
		case flagExtended:
			target = "omz"
		default:
			return fmt.Errorf("module argument required (%s)", modules.ValidModuleNames())
		}

		if _, ok := core.GetModule(target); !ok && target != "all" {
			return fmt.Errorf("unknown module %q (%s)", target, modules.ValidModuleNames())
		}

		if flagUpgrade {
			core.UpgradeMode = true
			upgradeClone(ctx, target == "all")
		}

		if core.IsSteamOS() && !core.DryRun {
			core.Info("SteamOS detected, disabling readonly filesystem...")
			if err := core.DisableReadonly(ctx); err != nil {
				return fmt.Errorf("failed to disable readonly mode: %w", err)
			}
			defer func() {
				core.Info("Re-enabling readonly filesystem...")
				core.EnableReadonly(ctx)
			}()
		}

		core.ExtendedMode = flagExtended
		core.ToolkitMode = flagToolkit

		// Run extended plugin menu before install (before spinner starts)
		if flagExtended {
			selected, err := modules.RunExtendedPluginMenu()
			if err != nil {
				return fmt.Errorf("extended plugin menu: %w", err)
			}
			core.Cfg.ExtendedPlugins = selected
		}

		// Apply registry override before toolkit operations
		if flagRegistry != "" {
			core.Cfg.ToolkitRegistryURL = flagRegistry
		}

		// Run toolkit menu before install (before spinner starts)
		if flagToolkit {
			selected, err := modules.RunToolkitMenu(ctx)
			if err != nil {
				return fmt.Errorf("toolkit menu: %w", err)
			}
			core.Cfg.ToolkitTools = selected
		}

		defer core.ReleaseSudo()

		if target == "all" {
			return installAll(ctx)
		}

		m, _ := core.GetModule(target)
		core.SetOptIn(target)
		return installOne(ctx, m)
	}

	installCmd := &cobra.Command{
		Use:   "install [module|all]",
		Short: "Install dotfile modules",
		Long: fmt.Sprintf("Install one or all dotfile modules.\nOmit module with --toolkit or --extended to install just that module.\n\n%s",
			modules.ValidModuleNames()),
		Args: cobra.MaximumNArgs(1),
		RunE: runInstall,
	}

	installCmd.Flags().BoolVar(&flagBackup, "backup", false, "Snapshot targets before modification (restorable)")
	installCmd.Flags().BoolVar(&flagExtended, "extended", false, "Interactive menu to select extended OMZ plugins")
	installCmd.Flags().BoolVar(&flagToolkit, "toolkit", false, "Interactive menu to select security/CTF/dev toolkit tools")
	installCmd.Flags().StringVar(&flagRegistry, "registry", "", "Path or URL to toolkit registry (overrides config)")
	installCmd.Flags().BoolVar(&flagUpgrade, "upgrade", false, "Pull the dotfiles clone and update tools and plugins dfinstall installed")

	var flagNoUpgrade bool
	updateCmd := &cobra.Command{
		Use:   "update [module|all]",
		Short: "Update dotfile modules (install --upgrade)",
		Long: fmt.Sprintf("Pull the dotfiles clone, update tools and plugins dfinstall installed, then re-apply one or all modules.\nThis is install --upgrade; --no-upgrade makes it a plain install.\nOmit module with --toolkit or --extended to install just that module.\n\n%s",
			modules.ValidModuleNames()),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			up, _ := cmd.Flags().GetBool("upgrade")
			flagUpgrade = up && !flagNoUpgrade
			return runInstall(cmd, args)
		},
	}

	updateCmd.Flags().BoolVar(&flagBackup, "backup", false, "Snapshot targets before modification (restorable)")
	updateCmd.Flags().BoolVar(&flagExtended, "extended", false, "Interactive menu to select extended OMZ plugins")
	updateCmd.Flags().BoolVar(&flagToolkit, "toolkit", false, "Interactive menu to select security/CTF/dev toolkit tools")
	updateCmd.Flags().StringVar(&flagRegistry, "registry", "", "Path or URL to toolkit registry (overrides config)")
	updateCmd.Flags().BoolVar(&flagNoUpgrade, "no-upgrade", false, "Re-apply modules without pulling the clone or updating tools")
	// Accepted so `update --upgrade` keeps working now that it is the default.
	updateCmd.Flags().Bool("upgrade", true, "")
	_ = updateCmd.Flags().MarkHidden("upgrade")

	var flagList bool

	restoreCmd := &cobra.Command{
		Use:   "restore [timestamp]",
		Short: "Restore files from a backup snapshot",
		Long:  "Restore files from a backup created with --backup.\nUse --list to see available backups.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagList {
				backups, err := core.ListBackups()
				if err != nil {
					return err
				}
				core.WriteBackupList(os.Stdout, backups)
				return nil
			}

			// Force verbose so restore messages are visible
			core.Level = core.LogVerbose

			var ts string
			if len(args) == 1 {
				ts = args[0]
			} else {
				// Resolve to latest
				backups, err := core.ListBackups()
				if err != nil {
					return err
				}
				if len(backups) == 0 {
					return fmt.Errorf("no backups found")
				}
				ts = backups[0].Timestamp
				core.Info("restoring latest backup: %s", ts)
			}

			res, err := core.RestoreBackup(ts)
			if err != nil {
				return err
			}
			// The dangling case is reported, not failed -- see RestoreBackup.
			core.Status("%s", res.Summary(ts))
			return nil
		},
	}

	restoreCmd.Flags().BoolVar(&flagList, "list", false, "List available backups")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of all modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			core.DetectEnvironment()
			if err := core.RequireDotfiles(); err != nil {
				return err
			}
			modules.PrintStatus()
			return nil
		},
	}

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Verify environment health",
		RunE: func(cmd *cobra.Command, args []string) error {
			core.DetectEnvironment()
			modules.RunDoctor()
			return nil
		},
	}

	rootSetupCmd := &cobra.Command{
		Use:   "root",
		Short: "Install root-owned copies of configs into /root/ via sudo",
		Long:  "Apply a curated subset of dotfiles (shell, git, nvim, tmux, htop) to the root user via sudo.\n\nThese are copies, not symlinks, so root never runs files your user can edit. Re-run after changing them.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if os.Geteuid() == 0 {
				core.Err("Do not run this command as root directly.")
				core.Err("Run as your normal user; sudo will be invoked automatically.")
				return fmt.Errorf("refusing to run as root")
			}

			core.DetectEnvironment()
			if err := core.RequireDotfiles(); err != nil {
				return err
			}

			defer core.ReleaseSudo()

			if core.Level >= core.LogVerbose {
				return modules.InstallRoot(ctx)
			}

			sp := core.NewSpinner()
			sp.Update("Installing root configs (sudo)")
			sp.Start()

			err := modules.InstallRoot(ctx)
			sp.Stop()

			core.FlushWarnings()

			if err != nil {
				core.Err("root: %v", err)
				return err
			}

			if core.DryRun {
				return nil
			}
			current, stale := modules.RootStatus(ctx)
			core.PrintResult(current+stale, stale)
			return nil
		},
	}

	// Add completions for install and update commands
	completeModules := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := append([]string{"all"}, core.ModuleNames()...)
		var matches []string
		for _, n := range names {
			if strings.HasPrefix(n, toComplete) {
				matches = append(matches, n)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}
	installCmd.ValidArgsFunction = completeModules
	updateCmd.ValidArgsFunction = completeModules

	uninstallCmd := &cobra.Command{
		Use:   "uninstall <module|all>",
		Short: "Remove symlinks created by dfinstall",
		Long: fmt.Sprintf("Uninstall one or all link-based modules.\n\n%s",
			modules.ValidModuleNames()),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			core.DetectEnvironment()
			if err := core.RequireDotfiles(); err != nil {
				return err
			}
			defer core.ReleaseSudo()
			target := args[0]
			if target == "all" {
				return uninstallAll(ctx)
			}
			m, ok := core.GetModule(target)
			if !ok {
				return fmt.Errorf("unknown module %q (%s)", target, modules.ValidModuleNames())
			}
			core.ClearOptIn(target)
			return uninstallOne(ctx, m)
		},
	}
	uninstallCmd.ValidArgsFunction = completeModules

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Show drift between config and filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			core.DetectEnvironment()
			if err := core.RequireDotfiles(); err != nil {
				return err
			}
			if err := core.LoadConfig(); err != nil {
				core.AlwaysWarn("%v", err)
			}
			if flagBrief {
				modules.CollectDiff().WriteBrief(os.Stdout)
				return nil
			}
			return runDiff()
		},
	}
	diffCmd.Flags().BoolVar(&flagBrief, "brief", false, "One line per link or managed file changed since install, nothing when clean (missing links are not listed)")

	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Toolkit registry utilities",
	}
	registryValidateCmd := &cobra.Command{
		Use:   "validate <path-or-url>",
		Short: "Validate a toolkit registry file (for CI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg, err := core.InspectRegistry(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%s registry valid (%d tools)\n", core.G.Ok, len(reg.Tools))
			return nil
		},
	}
	registryCmd.AddCommand(registryValidateCmd)

	var initOpts modules.InitOptions
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Clone the dotfiles checkout a released binary links from",
		Long: "Clone a dotfiles repository and record it as this machine's checkout, so a binary installed\n" +
			"without the repo (a release, a package, go install) has configs to link.\n\n" +
			"With no --dir and a checkout already recorded, init does nothing. --dir pointing at an\n" +
			"existing checkout adopts it without cloning. The default branch is the default ref:\n" +
			"'update' fast-forwards a branch, which a tag checkout does not have.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() == 0 {
				return fmt.Errorf("refusing to run as root; run init as your normal user")
			}
			_, err := modules.InitContent(cmd.Context(), initOpts)
			core.FlushWarnings()
			return err
		},
	}
	initCmd.Flags().StringVar(&initOpts.Repo, "repo", "", "repository to clone (default "+modules.DefaultContentRepo+")")
	initCmd.Flags().StringVar(&initOpts.Ref, "ref", "", "branch or tag to check out (default: the repository's default branch)")
	initCmd.Flags().StringVar(&initOpts.Dir, "dir", "", "where to put the checkout (default "+modules.DefaultContentDir()+")")

	rootCmd.AddCommand(initCmd, installCmd, updateCmd, statusCmd, doctorCmd, restoreCmd, rootSetupCmd, uninstallCmd, diffCmd, registryCmd)

	// Bind the command tree to a context canceled on SIGINT/SIGTERM, so a
	// Ctrl-C during a long apt or cargo run tears the child process down
	// instead of leaving it orphaned. A second signal restores the default
	// handler and kills us outright.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// upgradeClone is the first half of --upgrade: bring the clone and dfinstall
// up to date, and re-exec when dfinstall was replaced, so the install runs the
// new code.
func upgradeClone(ctx context.Context, all bool) {
	if os.Getenv(upgradedEnv) != "" {
		return
	}
	// install all adopts the clone it runs from; anything else links into
	// the canonical one.
	dir := core.DotfilesDir()
	if invoking := core.InvokingCloneDir(); all && invoking != "" {
		dir = invoking
	}
	exe := modules.UpgradeClone(ctx, dir)
	if exe == "" {
		return
	}
	os.Setenv(upgradedEnv, "1")
	err := syscall.Exec(exe, os.Args, os.Environ())
	core.AlwaysWarn("could not restart the updated dfinstall, continuing with this one: %v", err)
}

func installAll(ctx context.Context) error {
	sess, err := core.BeginInstall(core.InstallOptions{All: true, ForceBackup: flagBackup})
	if err != nil {
		return err
	}
	defer sess.Finish()

	if sess.CanonicalNow != "" {
		if sess.CanonicalPrev != "" {
			core.Status("Canonical dotfiles dir set to %s (was %s), repointing symlinks",
				sess.CanonicalNow, sess.CanonicalPrev)
		} else {
			core.Debug("canonical dotfiles dir set to %s", sess.CanonicalNow)
		}
	}

	all := core.AllModules()
	total := len(all)

	var skipped int
	var failures []string

	// Verbose/debug: full detailed output
	if core.Level >= core.LogVerbose {
		for _, m := range all {
			if core.SkipInAll(m.Name()) {
				core.Info("--- %s --- (skipped)", m.Name())
				sess.Skip(m.Name())
				skipped++
				continue
			}
			core.Info("--- %s ---", m.Name())
			if err := sess.Run(ctx, m); err != nil {
				core.Err("%s failed: %v", m.Name(), err)
				failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
			}
		}
		if len(failures) > 0 {
			sess.MarkFailed()
		}
		fmt.Println()
		sess.Report().Write(os.Stdout, true)
		core.Info("Done! Open a new terminal or run: exec zsh")
		return installFailure(failures)
	}

	// Default: spinner mode
	sp := core.NewSpinner()
	sp.Start()

	for i, m := range all {
		if core.SkipInAll(m.Name()) {
			sess.Skip(m.Name())
			skipped++
			continue
		}
		sp.Update("Installing %s (%d/%d)", m.Name(), i+1, total)
		core.Debug("starting module %s", m.Name())
		if err := sess.Run(ctx, m); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
		}
	}
	sp.Stop()

	if len(failures) > 0 {
		sess.MarkFailed()
	}

	core.FlushWarnings()
	for _, f := range failures {
		core.Err("%s", f)
	}

	sess.Report().Write(os.Stdout, false)
	if sess.DidBackup() {
		core.PrintHint("Backup saved; restore with: dfinstall restore")
	}
	core.PrintResult(total-skipped, len(failures))
	core.PrintHint("Open a new terminal or run: exec zsh")
	return installFailure(failures)
}

// installFailure turns collected per-module failures into a non-zero exit.
// The details are already on screen, so the returned error stays short.
func installFailure(failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%d module(s) failed", len(failures))
}

func installOne(ctx context.Context, m core.Module) error {
	// Under canonical-always semantics a partial install links into the
	// canonical clone, not necessarily the one you're sitting in. Say so, so it
	// isn't surprising — and point at how to switch.
	if canon := core.ReadCanonicalDir(); canon != "" {
		if invoking := core.InvokingCloneDir(); invoking != "" && invoking != canon {
			core.AlwaysWarn("running from %s, but canonical dotfiles dir is %s", invoking, canon)
			core.AlwaysWarn("links will target the canonical clone; run 'dfinstall install all' here to switch")
		}
	}

	sess, err := core.BeginInstall(core.InstallOptions{ForceBackup: flagBackup})
	if err != nil {
		return err
	}
	defer sess.Finish()

	// Verbose/debug: full detailed output
	if core.Level >= core.LogVerbose {
		err := sess.Run(ctx, m)
		fmt.Println()
		sess.Report().Write(os.Stdout, true)
		if err != nil {
			sess.MarkFailed()
			return err
		}
		return nil
	}

	// Default: spinner mode
	sp := core.NewSpinner()
	sp.Update("Installing %s", m.Name())
	sp.Start()

	installErr := sess.Run(ctx, m)
	sp.Stop()

	core.FlushWarnings()
	sess.Report().Write(os.Stdout, false)

	if installErr != nil {
		sess.MarkFailed()
		core.Err("%s: %v", m.Name(), installErr)
		return installErr
	}

	if sess.DidBackup() {
		core.PrintHint("Backup saved; restore with: dfinstall restore")
	}
	core.PrintResult(1, 0)
	return nil
}

func uninstallAll(ctx context.Context) error {
	all := core.AllModules()
	total := len(all)

	var failures []string

	if core.Level >= core.LogVerbose {
		for _, m := range all {
			u, ok := m.(core.Uninstaller)
			if !ok {
				core.Info("--- %s --- (no uninstall support)", m.Name())
				continue
			}
			core.Info("--- %s ---", m.Name())
			if err := u.Uninstall(ctx); err != nil {
				core.Err("%s: %v", m.Name(), err)
				failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
			}
		}
		return uninstallFailure(failures)
	}

	sp := core.NewSpinner()
	sp.Start()

	for i, m := range all {
		u, ok := m.(core.Uninstaller)
		if !ok {
			continue
		}
		sp.Update("Uninstalling %s (%d/%d)", m.Name(), i+1, total)
		if err := u.Uninstall(ctx); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
		}
	}
	sp.Stop()

	core.FlushWarnings()
	for _, f := range failures {
		core.Err("%s", f)
	}

	core.PrintResult(total, len(failures))
	return uninstallFailure(failures)
}

// uninstallFailure turns collected per-module failures into a non-zero exit.
func uninstallFailure(failures []string) error {
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%d module(s) failed to uninstall", len(failures))
}

func uninstallOne(ctx context.Context, m core.Module) error {
	u, ok := m.(core.Uninstaller)
	if !ok {
		core.Warn("%s does not support uninstall", m.Name())
		return nil
	}

	if core.Level >= core.LogVerbose {
		return u.Uninstall(ctx)
	}

	sp := core.NewSpinner()
	sp.Update("Uninstalling %s", m.Name())
	sp.Start()

	err := u.Uninstall(ctx)
	sp.Stop()

	core.FlushWarnings()

	if err != nil {
		core.Err("%s: %v", m.Name(), err)
		return err
	}

	core.PrintResult(1, 0)
	return nil
}

func runDiff() error {
	modules.CollectDiff().Write(os.Stdout,
		"run dfinstall install all to fix", "'dfinstall install all'")
	return nil
}
