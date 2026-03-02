package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/owenpierce/dotfiles/src/core"
	"github.com/owenpierce/dotfiles/src/modules"
	"github.com/spf13/cobra"
)

var (
	flagVerbose bool
	flagDebug   bool
	flagBackup  bool
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
			core.PrintBanner()
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Show debug output (implies verbose)")

	installCmd := &cobra.Command{
		Use:   "install <module|all>",
		Short: "Install dotfile modules",
		Long: fmt.Sprintf("Install one or all dotfile modules.\n\n%s",
			modules.ValidModuleNames()),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Geteuid() == 0 {
				core.Err("Running as root is not supported.")
				core.Err("Run as your normal user instead. To apply configs to /root, use:")
				core.Err("    dfinstall root")
				return fmt.Errorf("refusing to install as root")
			}

			core.DetectEnvironment()
			core.AssertEnvironment()

			target := args[0]

			if target == "all" {
				return installAll()
			}

			m, ok := core.GetModule(target)
			if !ok {
				return fmt.Errorf("unknown module %q — %s", target, modules.ValidModuleNames())
			}
			return installOne(m)
		},
	}

	installCmd.Flags().BoolVar(&flagBackup, "backup", false, "Snapshot targets before modification (restorable)")

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

	// Add completions for install command
	installCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

	rootCmd.AddCommand(installCmd, statusCmd, doctorCmd, restoreCmd, rootSetupCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func installAll() error {
	if flagBackup {
		if err := core.StartBackup(); err != nil {
			return fmt.Errorf("start backup: %w", err)
		}
		defer core.FinishBackup()
	}

	all := core.AllModules()
	total := len(all)

	// Verbose/debug: full detailed output
	if core.Level >= core.LogVerbose {
		for _, m := range all {
			core.Info("--- %s ---", m.Name())
			if err := m.Install(); err != nil {
				core.Err("%s failed: %v", m.Name(), err)
			}
		}
		fmt.Println()
		core.Info("Done! Open a new terminal or run: exec zsh")
		return nil
	}

	// Default: spinner mode
	sp := core.NewSpinner()
	sp.Start()

	var failures []string
	for i, m := range all {
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

	if flagBackup {
		core.PrintHint("Backup saved — restore with: dfinstall restore")
	}
	core.PrintResult(total, len(failures))
	core.PrintHint("Open a new terminal or run: exec zsh")
	return nil
}

func installOne(m core.Module) error {
	if flagBackup {
		if err := core.StartBackup(); err != nil {
			return fmt.Errorf("start backup: %w", err)
		}
		defer core.FinishBackup()
	}

	// Verbose/debug: full detailed output
	if core.Level >= core.LogVerbose {
		return m.Install()
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

	if flagBackup {
		core.PrintHint("Backup saved — restore with: dfinstall restore")
	}
	core.PrintResult(1, 0)
	return nil
}
