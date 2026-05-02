package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

var (
	flagVerbose  bool
	flagDebug    bool
	flagDryRun   bool
	flagBackup   bool
	flagExtended bool
	flagToolkit  bool
	flagRegistry string
)

func main() {
	modules.RegisterAllModules()

	rootCmd := &cobra.Command{
		Use:   "dfinstall",
		Short: "Dotfiles installer and manager",
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
			core.LoadConfig()
			core.PrintBanner()
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Show debug output (implies verbose)")
	rootCmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without modifying the filesystem")

	runInstall := func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() == 0 {
			core.Err("Running as root is not supported.")
			core.Err("Run as your normal user instead. To apply configs to /root, use:")
			core.Err("    dfinstall root")
			return fmt.Errorf("refusing to install as root")
		}

		core.DetectEnvironment()
		core.AssertEnvironment()

		if core.IsSteamOS() && !core.DryRun {
			core.Info("SteamOS detected — disabling readonly filesystem...")
			if err := core.DisableReadonly(); err != nil {
				return fmt.Errorf("failed to disable readonly mode: %w", err)
			}
			defer func() {
				core.Info("Re-enabling readonly filesystem...")
				core.EnableReadonly()
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
			selected, err := modules.RunToolkitMenu()
			if err != nil {
				return fmt.Errorf("toolkit menu: %w", err)
			}
			core.Cfg.ToolkitTools = selected
		}

		// Prompt for sudo before spinner starts so the password prompt is visible
		core.PromptSudo()
		defer core.StopSudoKeepAlive()

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
			return fmt.Errorf("module argument required — %s", modules.ValidModuleNames())
		}

		if target == "all" {
			return installAll()
		}

		m, ok := core.GetModule(target)
		if !ok {
			return fmt.Errorf("unknown module %q — %s", target, modules.ValidModuleNames())
		}
		return installOne(m)
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

	updateCmd := &cobra.Command{
		Use:   "update [module|all]",
		Short: "Update dotfile modules (alias for install)",
		Long: fmt.Sprintf("Re-apply one or all dotfile modules. This is an alias for install.\nOmit module with --toolkit or --extended to install just that module.\n\n%s",
			modules.ValidModuleNames()),
		Args: cobra.MaximumNArgs(1),
		RunE: runInstall,
	}

	updateCmd.Flags().BoolVar(&flagBackup, "backup", false, "Snapshot targets before modification (restorable)")
	updateCmd.Flags().BoolVar(&flagExtended, "extended", false, "Interactive menu to select extended OMZ plugins")
	updateCmd.Flags().BoolVar(&flagToolkit, "toolkit", false, "Interactive menu to select security/CTF/dev toolkit tools")
	updateCmd.Flags().StringVar(&flagRegistry, "registry", "", "Path or URL to toolkit registry (overrides config)")

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
				if len(backups) == 0 {
					fmt.Println("No backups found.")
					return nil
				}
				fmt.Printf("%-20s %s\n", "TIMESTAMP", "ENTRIES")
				for _, b := range backups {
					fmt.Printf("%-20s %d\n", b.Timestamp, b.Count)
				}
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

			return core.RestoreBackup(ts)
		},
	}

	restoreCmd.Flags().BoolVar(&flagList, "list", false, "List available backups")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of all modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			core.DetectEnvironment()
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
		Short: "Symlink configs into /root/ via sudo",
		Long:  "Apply a curated subset of dotfiles (shell, git, nvim, tmux, htop) to the root user via sudo.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() == 0 {
				core.Err("Do not run this command as root directly.")
				core.Err("Run as your normal user — sudo will be invoked automatically.")
				return fmt.Errorf("refusing to run as root")
			}

			core.DetectEnvironment()

			// Prime sudo before any spinner starts so the password prompt
			// (if any) is visible — this command is sudo-heavy.
			core.PromptSudo()
			defer core.StopSudoKeepAlive()

			if core.Level >= core.LogVerbose {
				return modules.InstallRoot()
			}

			sp := core.NewSpinner()
			sp.Update("Linking root configs (sudo)")
			sp.Start()

			err := modules.InstallRoot()
			sp.Stop()

			core.FlushWarnings()

			if err != nil {
				core.Err("root: %v", err)
				return err
			}

			linked, missing := modules.RootStatus()
			core.PrintResult(linked+missing, missing)
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
			core.DetectEnvironment()
			// Prime sudo — modules that installed system packages (delta,
			// toolkit) shell out to sudo when uninstalling.
			core.PromptSudo()
			defer core.StopSudoKeepAlive()
			target := args[0]
			if target == "all" {
				return uninstallAll()
			}
			m, ok := core.GetModule(target)
			if !ok {
				return fmt.Errorf("unknown module %q — %s", target, modules.ValidModuleNames())
			}
			return uninstallOne(m)
		},
	}
	uninstallCmd.ValidArgsFunction = completeModules

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Show drift between config and filesystem",
		RunE: func(cmd *cobra.Command, args []string) error {
			core.DetectEnvironment()
			core.LoadConfig()
			return runDiff()
		},
	}

	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Toolkit registry utilities",
	}
	registryValidateCmd := &cobra.Command{
		Use:   "validate <path-or-url>",
		Short: "Validate a toolkit registry file (for CI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := core.FetchRegistry(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("✓ registry valid (%d tools)\n", len(reg.Tools))
			return nil
		},
	}
	registryCmd.AddCommand(registryValidateCmd)

	rootCmd.AddCommand(installCmd, updateCmd, statusCmd, doctorCmd, restoreCmd, rootSetupCmd, uninstallCmd, diffCmd, registryCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func installAll() error {
	doBackup, firstRun := shouldBackup()

	if doBackup {
		if err := core.StartBackup(); err != nil {
			return fmt.Errorf("start backup: %w", err)
		}
		defer core.FinishBackup()
	}

	all := core.AllModules()
	total := len(all)

	// Verbose/debug: full detailed output
	var skipped int
	if core.Level >= core.LogVerbose {
		for _, m := range all {
			if core.IsModuleSkipped(m.Name()) {
				core.Info("--- %s --- (skipped)", m.Name())
				skipped++
				continue
			}
			core.Info("--- %s ---", m.Name())
			if err := m.Install(); err != nil {
				core.Err("%s failed: %v", m.Name(), err)
			}
		}
		fmt.Println()
		if firstRun {
			saveFirstRunConfig()
		} else if core.ExtendedMode || core.ToolkitMode {
			if err := core.SaveConfig(); err != nil {
				core.Warn("failed to save config: %v", err)
			}
		}
		core.Info("Done! Open a new terminal or run: exec zsh")
		return nil
	}

	// Default: spinner mode
	sp := core.NewSpinner()
	sp.Start()

	var failures []string
	for i, m := range all {
		if core.IsModuleSkipped(m.Name()) {
			skipped++
			continue
		}
		sp.Update("Installing %s (%d/%d)", m.Name(), i+1, total)
		core.Debug("starting module %s", m.Name())
		if err := m.Install(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
		}
	}
	sp.Stop()

	core.FlushWarnings()
	for _, f := range failures {
		core.Err("%s", f)
	}

	if firstRun {
		saveFirstRunConfig()
	} else if core.ExtendedMode || core.ToolkitMode {
		if err := core.SaveConfig(); err != nil {
			core.Warn("failed to save config: %v", err)
		}
	}
	if doBackup {
		core.PrintHint("Backup saved — restore with: dfinstall restore")
	}
	core.PrintResult(total-skipped, len(failures))
	core.PrintHint("Open a new terminal or run: exec zsh")
	return nil
}

func installOne(m core.Module) error {
	doBackup, firstRun := shouldBackup()

	if doBackup {
		if err := core.StartBackup(); err != nil {
			return fmt.Errorf("start backup: %w", err)
		}
		defer core.FinishBackup()
	}

	// Verbose/debug: full detailed output
	if core.Level >= core.LogVerbose {
		err := m.Install()
		if firstRun {
			saveFirstRunConfig()
		} else if core.ExtendedMode || core.ToolkitMode {
			if err := core.SaveConfig(); err != nil {
				core.Warn("failed to save config: %v", err)
			}
		}
		return err
	}

	// Default: spinner mode
	sp := core.NewSpinner()
	sp.Update("Installing %s", m.Name())
	sp.Start()

	err := m.Install()
	sp.Stop()

	core.FlushWarnings()

	if err != nil {
		core.Err("%s: %v", m.Name(), err)
		return err
	}

	if firstRun {
		saveFirstRunConfig()
	} else if core.ExtendedMode || core.ToolkitMode {
		if err := core.SaveConfig(); err != nil {
			core.Warn("failed to save config: %v", err)
		}
	}
	if doBackup {
		core.PrintHint("Backup saved — restore with: dfinstall restore")
	}
	core.PrintResult(1, 0)
	return nil
}

// shouldBackup decides whether to create a backup and whether this is a first run.
func shouldBackup() (doBackup bool, firstRun bool) {
	if core.DryRun {
		return false, false
	}

	// --backup flag always wins
	if flagBackup {
		return true, false
	}

	// No config file → first run, auto-backup
	if !core.CfgFileExists {
		core.Info("first run detected — creating automatic backup")
		return true, true
	}

	// Config exists — respect skip_backup preference
	if !core.Cfg.SkipBackup {
		return true, false
	}

	return false, false
}

// saveFirstRunConfig writes the config with skip_backup: true after first-run auto-backup.
func saveFirstRunConfig() {
	if core.DryRun {
		return
	}
	core.Cfg.SkipBackup = true
	if err := core.SaveConfig(); err != nil {
		core.Warn("failed to save config: %v", err)
	} else {
		core.Info("config saved: %s (skip_backup: true)", core.ConfigFilePath())
	}
}

func uninstallAll() error {
	all := core.AllModules()
	total := len(all)

	if core.Level >= core.LogVerbose {
		for _, m := range all {
			u, ok := m.(core.Uninstaller)
			if !ok {
				core.Info("--- %s --- (no uninstall support)", m.Name())
				continue
			}
			core.Info("--- %s ---", m.Name())
			if err := u.Uninstall(); err != nil {
				core.Err("%s: %v", m.Name(), err)
			}
		}
		return nil
	}

	sp := core.NewSpinner()
	sp.Start()

	var failures []string
	for i, m := range all {
		u, ok := m.(core.Uninstaller)
		if !ok {
			continue
		}
		sp.Update("Uninstalling %s (%d/%d)", m.Name(), i+1, total)
		if err := u.Uninstall(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
		}
	}
	sp.Stop()

	core.FlushWarnings()
	for _, f := range failures {
		core.Err("%s", f)
	}

	core.PrintResult(total, len(failures))
	return nil
}

func uninstallOne(m core.Module) error {
	u, ok := m.(core.Uninstaller)
	if !ok {
		core.Warn("%s does not support uninstall", m.Name())
		return nil
	}

	if core.Level >= core.LogVerbose {
		return u.Uninstall()
	}

	sp := core.NewSpinner()
	sp.Update("Uninstalling %s", m.Name())
	sp.Start()

	err := u.Uninstall()
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
	all := core.AllModules()
	var issues int

	for _, m := range all {
		if core.IsModuleSkipped(m.Name()) {
			fmt.Printf("%-15s  skipped\n", m.Name())
			continue
		}

		if le, ok := m.(core.LinkExporter); ok {
			links := le.Links()
			modOk := true
			for _, lp := range links {
				status := core.CheckLink(lp.Src, lp.Dst)
				if status != "ok" {
					if modOk {
						fmt.Printf("%-15s\n", m.Name())
						modOk = false
					}
					switch status {
					case "missing":
						fmt.Printf("  missing: %s\n", lp.Dst)
					case "wrong":
						fmt.Printf("  wrong target: %s\n", lp.Dst)
					case "file":
						fmt.Printf("  regular file (not symlinked): %s\n", lp.Dst)
					}
					issues++
				}
			}
			if modOk {
				fmt.Printf("%-15s  ok (%d links)\n", m.Name(), len(links))
			}
		} else {
			s := m.Status()
			if s.Missing > 0 {
				fmt.Printf("%-15s  %d missing\n", m.Name(), s.Missing)
				issues += s.Missing
			} else {
				extra := ""
				if s.Extra != "" {
					extra = " (" + s.Extra + ")"
				}
				fmt.Printf("%-15s  ok%s\n", m.Name(), extra)
			}
		}
	}

	fmt.Println()
	if issues == 0 {
		fmt.Println("No drift detected.")
	} else {
		fmt.Printf("%d issue(s) — run dfinstall install all to fix\n", issues)
	}
	return nil
}
