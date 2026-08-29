package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sresarehumantoo/dotfiles/src/core"
	"github.com/sresarehumantoo/dotfiles/src/modules"
)

func main() {
	// Save real stdout for MCP JSON-RPC transport.
	// All fmt.Printf (used by core output functions) will go to stderr.
	realStdout := os.Stdout
	os.Stdout = os.Stderr

	core.Level = core.LogQuiet

	// os.Stdin carries the JSON-RPC request stream here, so nothing may read it
	// looking for user input — a prompt would consume protocol bytes and wedge
	// the session. Interactive paths check this and take a safe default.
	core.Interactive = false

	core.DetectEnvironment()
	if err := core.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "dfinstall: %v\n", err)
	}
	modules.RegisterAllModules()

	s := server.NewMCPServer("dfinstall", core.Version)
	registerTools(s)

	stdioServer := server.NewStdioServer(s)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := stdioServer.Listen(ctx, os.Stdin, realStdout); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		os.Exit(1)
	}
}

func registerTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("dfinstall_status",
			mcp.WithDescription("Show install status of all dotfile modules (linked/missing symlinks)"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleStatus,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_install",
			mcp.WithDescription("Install a dotfile module or all modules. Creates symlinks and installs packages."),
			mcp.WithString("module",
				mcp.Required(),
				mcp.Description("Module name to install, or 'all' for everything"),
			),
			mcp.WithIdempotentHintAnnotation(true),
		),
		handleInstall,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_doctor",
			mcp.WithDescription("Run health checks on the dotfiles environment (tools, configs, symlinks)"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleDoctor,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_list_modules",
			mcp.WithDescription("List all available dotfile modules in install order"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleListModules,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_list_backups",
			mcp.WithDescription("List available backup snapshots that can be restored"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleListBackups,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_restore",
			mcp.WithDescription("Restore files from a backup snapshot"),
			mcp.WithString("timestamp",
				mcp.Description("Backup timestamp to restore (latest if omitted)"),
			),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleRestore,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_uninstall",
			mcp.WithDescription("Remove symlinks for a dotfile module or all modules"),
			mcp.WithString("module",
				mcp.Required(),
				mcp.Description("Module name to uninstall, or 'all' for everything"),
			),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleUninstall,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_diff",
			mcp.WithDescription("Show drift between config and filesystem"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleDiff,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_config",
			mcp.WithDescription("Read or write dfinstall configuration"),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("'get' to read config, 'set' to write a value"),
			),
			mcp.WithString("key",
				mcp.Description("Config key: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url, windev_enabled"),
			),
			mcp.WithString("value",
				mcp.Description("Value to set (required for 'set' action)"),
			),
		),
		handleConfig,
	)

	s.AddTool(
		mcp.NewTool("dfinstall_registry_validate",
			mcp.WithDescription("Validate a toolkit registry file and report the tool count. Accepts an HTTP(S) URL, file:// URL, or local path."),
			mcp.WithString("source",
				mcp.Required(),
				mcp.Description("Path or URL to the toolkit registry to validate"),
			),
		),
		handleRegistryValidate,
	)
}

// --- Tool handlers ---

func handleStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var b strings.Builder
	modules.WriteStatus(&b)
	return mcp.NewToolResultText(b.String()), nil
}

func handleInstall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("module", "")
	if name == "" {
		return mcp.NewToolResultError("module parameter is required"), nil
	}

	if name == "all" {
		// Share the CLI's session: adopts the canonical clone (repointing
		// stray symlinks), takes a restorable backup, and persists config.
		sess, err := core.BeginInstall(core.InstallOptions{All: true})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer sess.Finish()

		// Prime sudo so package steps don't each hit an unprimed sudo. With
		// Interactive=false this only uses cached or known credentials and
		// never prompts.
		core.PromptSudo(ctx)
		defer core.StopSudoKeepAlive()

		var b strings.Builder
		if sess.CanonicalNow != "" && sess.CanonicalPrev != "" {
			fmt.Fprintf(&b, "Canonical dotfiles dir set to %s (was %s) — repointing symlinks\n\n", sess.CanonicalNow, sess.CanonicalPrev)
		}

		beforeStatus := make(map[string]core.ModuleStatus)
		for _, m := range core.AllModules() {
			beforeStatus[m.Name()] = m.Status()
		}

		var failures []string
		for _, m := range core.AllModules() {
			// core.SkipInAll, not IsModuleSkipped: the latter ignores the
			// windev opt-in and installed it on machines that never enabled it.
			if core.SkipInAll(m.Name()) {
				continue
			}
			if err := m.Install(ctx); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
			}
		}

		for _, m := range core.AllModules() {
			after := m.Status()
			before := beforeStatus[m.Name()]
			fixed := before.Missing - after.Missing
			if fixed > 0 {
				fmt.Fprintf(&b, "%s: fixed %d missing links\n", after.Name, fixed)
			} else if after.Missing == 0 {
				fmt.Fprintf(&b, "%s: ok (%d linked)\n", after.Name, after.Linked)
			} else {
				fmt.Fprintf(&b, "%s: %d still missing\n", after.Name, after.Missing)
			}
		}

		if len(failures) > 0 {
			sess.MarkFailed()
			fmt.Fprintf(&b, "\nFailures:\n")
			for _, f := range failures {
				fmt.Fprintf(&b, "  - %s\n", f)
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}

	m, ok := core.GetModule(name)
	if !ok {
		return mcp.NewToolResultError(
			fmt.Sprintf("unknown module %q — valid: %s", name, strings.Join(core.ModuleNames(), ", ")),
		), nil
	}

	if name == "windev" {
		core.SetWindevOptIn()
	}

	sess, err := core.BeginInstall(core.InstallOptions{})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer sess.Finish()

	core.PromptSudo(ctx)
	defer core.StopSudoKeepAlive()

	before := m.Status()
	err = m.Install(ctx)
	after := m.Status()

	var b strings.Builder
	if err != nil {
		sess.MarkFailed()
		fmt.Fprintf(&b, "install %s error: %v\n", name, err)
	}
	fmt.Fprintf(&b, "before: %d linked, %d missing\n", before.Linked, before.Missing)
	fmt.Fprintf(&b, "after:  %d linked, %d missing\n", after.Linked, after.Missing)

	if err != nil {
		return mcp.NewToolResultError(b.String()), nil
	}
	return mcp.NewToolResultText(b.String()), nil
}

func handleDoctor(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Render the shared check set (modules.RunDoctorChecks) so the MCP doctor
	// stays in lockstep with the CLI `doctor` command.
	var b strings.Builder
	allOk := true
	for _, r := range modules.RunDoctorChecks() {
		if r.OK {
			// ⚠ The detail is printed on PASSING rows too, exactly as the CLI
			// does. Not cosmetic: "clone freshness" never fails by design, so
			// its detail IS its entire output — dropping it here made the
			// check invisible through this surface while the CLI showed it.
			if r.Detail != "" {
				fmt.Fprintf(&b, "  ok  %s: %s\n", r.Name, r.Detail)
			} else {
				fmt.Fprintf(&b, "  ok  %s\n", r.Name)
			}
		} else {
			fmt.Fprintf(&b, "  FAIL  %s - %s\n", r.Name, r.Detail)
			for _, e := range r.Extra {
				fmt.Fprintf(&b, "        %s\n", e)
			}
			allOk = false
		}
	}

	fmt.Fprintln(&b)
	if allOk {
		fmt.Fprintln(&b, "All checks passed!")
	} else {
		fmt.Fprintln(&b, "Some checks failed. Use dfinstall_install to fix.")
	}

	return mcp.NewToolResultText(b.String()), nil
}

func handleListModules(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	names := core.ModuleNames()
	var b strings.Builder
	for i, name := range names {
		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
	}
	return mcp.NewToolResultText(b.String()), nil
}

func handleListBackups(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	backups, err := core.ListBackups()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list backups: %v", err)), nil
	}
	if len(backups) == 0 {
		return mcp.NewToolResultText("No backups found."), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %s\n", "TIMESTAMP", "ENTRIES")
	for _, bk := range backups {
		fmt.Fprintf(&b, "%-20s %d\n", bk.Timestamp, bk.Count)
	}
	return mcp.NewToolResultText(b.String()), nil
}

func handleRestore(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ts := request.GetString("timestamp", "")

	if ts == "" {
		backups, err := core.ListBackups()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list backups: %v", err)), nil
		}
		if len(backups) == 0 {
			return mcp.NewToolResultError("no backups found"), nil
		}
		ts = backups[0].Timestamp
	}

	if err := core.RestoreBackup(ts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("restore %s: %v", ts, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Restored backup %s successfully.", ts)), nil
}

func handleConfig(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := request.GetString("action", "")
	key := request.GetString("key", "")
	value := request.GetString("value", "")

	switch action {
	case "get":
		if key == "" {
			var b strings.Builder
			fmt.Fprintf(&b, "skip_backup: %v\n", core.Cfg.SkipBackup)
			fmt.Fprintf(&b, "backup_dir: %s\n", core.Cfg.BackupDirP)
			fmt.Fprintf(&b, "extended_plugins: %v\n", core.Cfg.ExtendedPlugins)
			fmt.Fprintf(&b, "preserved_files: %v\n", core.Cfg.PreservedFiles)
			fmt.Fprintf(&b, "dismissed_files: %v\n", core.Cfg.DismissedFiles)
			fmt.Fprintf(&b, "skip_modules: %v\n", core.Cfg.SkipModules)
			fmt.Fprintf(&b, "toolkit_tools: %v\n", core.Cfg.ToolkitTools)
			fmt.Fprintf(&b, "toolkit_registry_url: %s\n", core.Cfg.ToolkitRegistryURL)
			fmt.Fprintf(&b, "windev_enabled: %v\n", core.Cfg.WindevEnabled)
			fmt.Fprintf(&b, "\nconfig file: %s\n", core.ConfigFilePath())
			return mcp.NewToolResultText(b.String()), nil
		}
		switch key {
		case "skip_backup":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.SkipBackup)), nil
		case "backup_dir":
			dir := core.Cfg.BackupDirP
			if dir == "" {
				dir = core.BackupDir()
			}
			return mcp.NewToolResultText(dir), nil
		case "extended_plugins":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.ExtendedPlugins)), nil
		case "preserved_files":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.PreservedFiles)), nil
		case "dismissed_files":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.DismissedFiles)), nil
		case "skip_modules":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.SkipModules)), nil
		case "toolkit_tools":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.ToolkitTools)), nil
		case "toolkit_registry_url":
			url := core.Cfg.ToolkitRegistryURL
			if url == "" {
				url = core.DefaultRegistryURL
			}
			return mcp.NewToolResultText(url), nil
		case "windev_enabled":
			return mcp.NewToolResultText(fmt.Sprintf("%v", core.Cfg.WindevEnabled)), nil
		default:
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown config key: %s (valid: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url, windev_enabled)", key),
			), nil
		}

	case "set":
		if key == "" {
			return mcp.NewToolResultError("key is required for 'set' action"), nil
		}
		switch key {
		case "skip_backup":
			core.Cfg.SkipBackup = value == "true"
		case "backup_dir":
			core.Cfg.BackupDirP = value
		case "extended_plugins":
			if value == "" {
				core.Cfg.ExtendedPlugins = nil
			} else {
				core.Cfg.ExtendedPlugins = strings.Split(value, ",")
			}
		case "preserved_files":
			if value == "" {
				core.Cfg.PreservedFiles = nil
			} else {
				core.Cfg.PreservedFiles = strings.Split(value, ",")
			}
		case "dismissed_files":
			if value == "" {
				core.Cfg.DismissedFiles = nil
			} else {
				core.Cfg.DismissedFiles = strings.Split(value, ",")
			}
		case "skip_modules":
			if value == "" {
				core.Cfg.SkipModules = nil
			} else {
				core.Cfg.SkipModules = strings.Split(value, ",")
			}
		case "toolkit_tools":
			if value == "" {
				core.Cfg.ToolkitTools = nil
			} else {
				core.Cfg.ToolkitTools = strings.Split(value, ",")
			}
		case "toolkit_registry_url":
			core.Cfg.ToolkitRegistryURL = value
		case "windev_enabled":
			core.Cfg.WindevEnabled = value == "true"
		default:
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown config key: %s (valid: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url, windev_enabled)", key),
			), nil
		}
		if err := core.SaveConfig(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("save config: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Set %s = %s", key, value)), nil

	default:
		return mcp.NewToolResultError("action must be 'get' or 'set'"), nil
	}
}

func handleRegistryValidate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := request.GetString("source", "")
	if source == "" {
		return mcp.NewToolResultError("source parameter is required"), nil
	}
	reg, err := core.InspectRegistry(ctx, source)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid registry: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("registry valid (%d tools)", len(reg.Tools))), nil
}

func handleUninstall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("module", "")
	if name == "" {
		return mcp.NewToolResultError("module parameter is required"), nil
	}

	if name == "all" {
		core.PromptSudo(ctx)
		defer core.StopSudoKeepAlive()

		var b strings.Builder
		for _, m := range core.AllModules() {
			u, ok := m.(core.Uninstaller)
			if !ok {
				fmt.Fprintf(&b, "%s: no uninstall support\n", m.Name())
				continue
			}
			before := m.Status()
			if err := u.Uninstall(ctx); err != nil {
				fmt.Fprintf(&b, "%s: error: %v\n", m.Name(), err)
				continue
			}
			after := m.Status()
			removed := before.Linked - after.Linked
			fmt.Fprintf(&b, "%s: removed %d links\n", m.Name(), removed)
		}
		return mcp.NewToolResultText(b.String()), nil
	}

	m, ok := core.GetModule(name)
	if !ok {
		return mcp.NewToolResultError(
			fmt.Sprintf("unknown module %q — valid: %s", name, strings.Join(core.ModuleNames(), ", ")),
		), nil
	}

	u, uOk := m.(core.Uninstaller)
	if !uOk {
		return mcp.NewToolResultError(fmt.Sprintf("%s does not support uninstall", name)), nil
	}

	// Modules that installed system packages (delta, toolkit) shell out to
	// sudo when uninstalling.
	core.PromptSudo(ctx)
	defer core.StopSudoKeepAlive()

	// Read the before-state first: WindevModule.Status() short-circuits to
	// "disabled" once the opt-in is cleared, so clearing first reported every
	// windev uninstall as 0 links removed no matter how many it deleted.
	before := m.Status()

	if name == "windev" {
		core.ClearWindevOptIn()
	}

	if err := u.Uninstall(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("uninstall %s: %v", name, err)), nil
	}
	after := m.Status()

	var b strings.Builder
	fmt.Fprintf(&b, "before: %d linked, %d missing\n", before.Linked, before.Missing)
	fmt.Fprintf(&b, "after:  %d linked, %d missing\n", after.Linked, after.Missing)
	return mcp.NewToolResultText(b.String()), nil
}

func handleDiff(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var b strings.Builder
	modules.CollectDiff().Write(&b,
		"run dfinstall_install with module 'all' to fix",
		"dfinstall_install with module 'all'")
	return mcp.NewToolResultText(b.String()), nil
}
