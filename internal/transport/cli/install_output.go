package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/i18n"
	"golang.org/x/term"
)

type installResult struct {
	root        string
	agents      []string
	manifest    integration.Manifest
	changed     bool
	changes     []integration.FileChange
	hooksPlan   integration.HooksPlan
	diagnostics map[string]integration.Diagnostic
}

func installColor(output io.Writer) bool {
	f, ok := output.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
}

func installAgentName(agent string) string {
	switch agent {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude Code"
	case "codebuddy":
		return "CodeBuddy"
	default:
		return agent
	}
}

func installDisplayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return rel
	}
	return path
}

func writeInstallResult(ctx context.Context, output io.Writer, result installResult, color bool) error {
	var b strings.Builder
	t := func(message *goi18n.Message) string { return i18n.TContext(ctx, message) }
	paint := func(code, text string) string {
		if !color || code == "" {
			return text
		}
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	heading := func(message *goi18n.Message) { fmt.Fprintf(&b, "\n  %s\n", paint("1;36", t(message))) }
	path := func(label string) { fmt.Fprintf(&b, "      %s\n", paint("2", label)) }
	title := msgInstallComplete
	if !result.changed {
		title = msgInstallUnchanged
	}
	fmt.Fprintf(&b, "\n%s %s\n", paint("32", "✓"), paint("1", t(title)))
	heading(msgInstallAgentConfigs)
	var agents []string
	seen := map[string]bool{}
	for _, agent := range result.agents {
		if seen[agent] {
			continue
		}
		seen[agent] = true
		agents = append(agents, agent)
		config := result.manifest.Config.Path
		if agent == "claude" && result.manifest.Claude != nil {
			config = result.manifest.Claude.Path
		}
		if agent == "codebuddy" && result.manifest.CodeBuddy != nil {
			config = result.manifest.CodeBuddy.Path
		}
		fmt.Fprintf(&b, "    %s %s\n", paint("32", "✓"), installAgentName(agent))
		path(installDisplayPath(result.root, config))
	}
	if len(result.manifest.Hooks) > 0 {
		heading(msgInstallGitHooks)
		groups := map[string][]integration.File{}
		var directories []string
		for _, hook := range result.manifest.Hooks {
			dir := filepath.Dir(hook.Path)
			if _, ok := groups[dir]; !ok {
				directories = append(directories, dir)
			}
			groups[dir] = append(groups[dir], hook)
		}
		for _, dir := range directories {
			for _, hook := range groups[dir] {
				fmt.Fprintf(&b, "    %s %s\n", paint("32", "✓"), filepath.Base(hook.Path))
			}
			path(t(msgInstallDirectory) + ": " + installDisplayPath(result.root, dir) + string(filepath.Separator))
		}
	}
	changedPaths := map[string]bool{}
	for _, change := range result.changes {
		if change.Action != "remove" {
			changedPaths[change.Path] = true
		}
	}
	backupsHeading := false
	for _, hook := range result.manifest.Hooks {
		if hook.Backup == "" || !changedPaths[hook.Backup] {
			continue
		}
		if !backupsHeading {
			heading(msgInstallBackups)
			backupsHeading = true
		}
		fmt.Fprintf(&b, "    %s %s\n", paint("32", "✓"), filepath.Base(hook.Path))
		path(installDisplayPath(result.root, hook.Backup))
	}
	heading(msgInstallRuntimeStatus)
	for _, agent := range agents {
		writeAgentDiagnostic(ctx, &b, agent, result.diagnostics[agent], color)
	}
	if result.hooksPlan.Override {
		heading(msgInstallNotes)
		fmt.Fprintf(&b, "    %s %s\n", paint("33", "!"), t(msgInstallInherited))
		path(result.hooksPlan.Inherited + " → " + installDisplayPath(result.root, result.hooksPlan.Path))
	}
	b.WriteByte('\n')
	_, err := io.WriteString(output, b.String())
	return err
}

var (
	msgInstallComplete         = &goi18n.Message{ID: "commands.InstallComplete", Other: "Installation complete"}
	msgInstallUnchanged        = &goi18n.Message{ID: "commands.InstallUnchanged", Other: "Already configured"}
	msgInstallAgentConfigs     = &goi18n.Message{ID: "commands.InstallAgentConfigs", Other: "Agent configuration"}
	msgInstallGitHooks         = &goi18n.Message{ID: "commands.InstallGitHooks", Other: "Git hooks"}
	msgInstallDirectory        = &goi18n.Message{ID: "commands.InstallDirectory", Other: "Directory"}
	msgInstallBackups          = &goi18n.Message{ID: "commands.InstallBackups", Other: "Original hook backups"}
	msgInstallRuntimeStatus    = &goi18n.Message{ID: "commands.InstallRuntimeStatus", Other: "Runtime status"}
	msgInstallNotes            = &goi18n.Message{ID: "commands.InstallNotes", Other: "Notes"}
	msgInstallActive           = &goi18n.Message{ID: "commands.InstallActive", Other: "Loaded and allowed to execute"}
	msgInstallPending          = &goi18n.Message{ID: "commands.InstallPending", Other: "Configured; restart the client to apply"}
	msgInstallInactive         = &goi18n.Message{ID: "commands.InstallInactive", Other: "Hooks are not fully active"}
	msgInstallUnavailable      = &goi18n.Message{ID: "commands.InstallUnavailable", Other: "Runtime not verified"}
	msgInstallConflict         = &goi18n.Message{ID: "commands.InstallConflict", Other: "Integration needs attention"}
	msgInstallUnknown          = &goi18n.Message{ID: "commands.InstallUnknown", Other: "Runtime status unknown"}
	msgInstallSessionOverrides = &goi18n.Message{ID: "commands.InstallSessionOverrides", Other: "Session overrides may still affect execution."}
	msgInstallInherited        = &goi18n.Message{ID: "commands.InstallInherited", Other: "Inherited hooks no longer run in this repository."}
)
