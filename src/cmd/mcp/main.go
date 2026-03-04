package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	core.DetectEnvironment()
	core.LoadConfig()
	modules.RegisterAllModules()

	s := server.NewMCPServer("dfinstall", "1.0.0")
	registerTools(s)

	stdioServer := server.NewStdioServer(s)
	ctx := context.Background()
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
				mcp.Description("Config key: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url"),
			),
			mcp.WithString("value",
				mcp.Description("Value to set (required for 'set' action)"),
			),
		),
		handleConfig,
	)
}

// --- Tool handlers ---

func handleStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%-15s  %7s  %7s  %s\n", "MODULE", "LINKED", "MISSING", "INFO")
	fmt.Fprintf(&b, "%-15s  %7s  %7s  %s\n", "------", "------", "-------", "----")

	for _, m := range core.AllModules() {
		s := m.Status()
		if core.IsModuleSkipped(m.Name()) {
			if s.Extra != "" {
				s.Extra += ", skipped"
			} else {
				s.Extra = "skipped"
			}
		}
		fmt.Fprintf(&b, "%-15s  %7d  %7d  %s\n", s.Name, s.Linked, s.Missing, s.Extra)
	}
	return mcp.NewToolResultText(b.String()), nil
}

func handleInstall(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("module", "")
	if name == "" {
		return mcp.NewToolResultError("module parameter is required"), nil
	}

	if name == "all" {
		beforeStatus := make(map[string]core.ModuleStatus)
		for _, m := range core.AllModules() {
			beforeStatus[m.Name()] = m.Status()
		}

		var failures []string
		for _, m := range core.AllModules() {
			if core.IsModuleSkipped(m.Name()) {
				continue
			}
			if err := m.Install(); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", m.Name(), err))
			}
		}

		var b strings.Builder
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

	before := m.Status()
	err := m.Install()
	after := m.Status()

	var b strings.Builder
	if err != nil {
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
	home, _ := os.UserHomeDir()

	type check struct {
		name string
		fn   func() string
	}

	checks := []check{
		{"go", cmdCheck("go")},
		{"nvim", cmdCheck("nvim")},
		{"zsh", cmdCheck("zsh")},
		{"tmux", cmdCheck("tmux")},
		{"git", cmdCheck("git")},
		{"delta", cmdCheck("delta")},
		{"curl", cmdCheck("curl")},
		{"fzf", cmdCheck("fzf")},
		{"ripgrep", cmdCheck("rg")},
		{"docker", cmdCheck("docker")},
		{"terraform", cmdCheck("terraform")},
		{"pip3", cmdCheck("pip3")},
		{"oh-my-zsh", dirCheck(filepath.Join(home, ".oh-my-zsh"))},
		{"zsh-autosuggestions", dirCheck(filepath.Join(home, ".oh-my-zsh", "custom", "plugins", "zsh-autosuggestions"))},
		{"powerlevel10k", dirCheck(filepath.Join(home, ".oh-my-zsh", "custom", "themes", "powerlevel10k"))},
		{"fonts", fileCheck(filepath.Join(home, ".local", "share", "fonts", "HackNerdFont-Regular.ttf"))},
		{"nvim config", linkCheck(
			core.ConfigPath("nvim", "init.lua"),
			filepath.Join(core.XDGConfigHome(), "nvim", "init.lua"),
		)},
		{"shell config", linkCheck(
			core.ConfigPath("shell", "zshrc"),
			filepath.Join(home, ".zshrc"),
		)},
		{"git config", linkCheck(
			core.ConfigPath("git", "gitconfig"),
			filepath.Join(home, ".gitconfig"),
		)},
		{"tmux config", linkCheck(
			core.ConfigPath("tmux", "tmux.conf"),
			filepath.Join(core.XDGConfigHome(), "tmux", "tmux.conf"),
		)},
	}

	if core.IsWSL() {
		checks = append(checks,
			check{"wsl.conf", fileMatchCheck(
				core.ConfigPath("wsl", "wsl.conf"),
				"/etc/wsl.conf",
			)},
		)
	}

	var b strings.Builder
	allOk := true
	for _, c := range checks {
		if msg := c.fn(); msg == "" {
			fmt.Fprintf(&b, "  ok  %s\n", c.name)
		} else {
			fmt.Fprintf(&b, "  FAIL  %s - %s\n", c.name, msg)
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
		default:
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown config key: %s (valid: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url)", key),
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
		default:
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown config key: %s (valid: skip_backup, backup_dir, extended_plugins, preserved_files, dismissed_files, skip_modules, toolkit_tools, toolkit_registry_url)", key),
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

func handleUninstall(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("module", "")
	if name == "" {
		return mcp.NewToolResultError("module parameter is required"), nil
	}

	if name == "all" {
		var b strings.Builder
		for _, m := range core.AllModules() {
			before := m.Status()
			u, ok := m.(core.Uninstaller)
			if !ok {
				fmt.Fprintf(&b, "%s: no uninstall support\n", m.Name())
				continue
			}
			if err := u.Uninstall(); err != nil {
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

	before := m.Status()
	if err := u.Uninstall(); err != nil {
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
	var issues int

	for _, m := range core.AllModules() {
		if core.IsModuleSkipped(m.Name()) {
			fmt.Fprintf(&b, "%-15s  skipped\n", m.Name())
			continue
		}

		if le, ok := m.(core.LinkExporter); ok {
			links := le.Links()
			modOk := true
			for _, lp := range links {
				status := core.CheckLink(lp.Src, lp.Dst)
				if status != "ok" {
					if modOk {
						fmt.Fprintf(&b, "%-15s\n", m.Name())
						modOk = false
					}
					switch status {
					case "missing":
						fmt.Fprintf(&b, "  missing: %s\n", lp.Dst)
					case "wrong":
						fmt.Fprintf(&b, "  wrong target: %s\n", lp.Dst)
					case "file":
						fmt.Fprintf(&b, "  regular file (not symlinked): %s\n", lp.Dst)
					}
					issues++
				}
			}
			if modOk {
				fmt.Fprintf(&b, "%-15s  ok (%d links)\n", m.Name(), len(links))
			}
		} else {
			s := m.Status()
			if s.Missing > 0 {
				fmt.Fprintf(&b, "%-15s  %d missing\n", m.Name(), s.Missing)
				issues += s.Missing
			} else {
				extra := ""
				if s.Extra != "" {
					extra = " (" + s.Extra + ")"
				}
				fmt.Fprintf(&b, "%-15s  ok%s\n", m.Name(), extra)
			}
		}
	}

	fmt.Fprintln(&b)
	if issues == 0 {
		fmt.Fprintln(&b, "No drift detected.")
	} else {
		fmt.Fprintf(&b, "%d issue(s) — run dfinstall_install with module 'all' to fix\n", issues)
	}
	return mcp.NewToolResultText(b.String()), nil
}

// --- Doctor check helpers ---

func cmdCheck(name string) func() string {
	return func() string {
		if _, err := exec.LookPath(name); err != nil {
			return "not found"
		}
		return ""
	}
}

func dirCheck(path string) func() string {
	return func() string {
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			return "not found"
		}
		return ""
	}
}

func fileCheck(path string) func() string {
	return func() string {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "not found"
		}
		return ""
	}
}

func linkCheck(src, dst string) func() string {
	return func() string {
		switch core.CheckLink(src, dst) {
		case "ok":
			return ""
		case "wrong":
			return "wrong target"
		case "file":
			return "regular file (not symlinked)"
		default:
			return "not found"
		}
	}
}

func fileMatchCheck(src, dst string) func() string {
	return func() string {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return "not found"
		}
		if !core.FilesMatch(src, dst) {
			return "outdated"
		}
		return ""
	}
}
