package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/capture"
	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/app/review"
	"github.com/yhlooo/motif/internal/domain"
	"github.com/yhlooo/motif/internal/transport/web"
)

func dataDir() (string, error) {
	if p := os.Getenv("MOTIF_DATA_DIR"); p != "" {
		return p, nil
	}
	h, e := os.UserHomeDir()
	return filepath.Join(h, ".motif"), e
}
func openStore() (*sqlite.Store, error) {
	p, e := dataDir()
	if e != nil {
		return nil, e
	}
	return sqlite.Open(p)
}
func openProject(cmd *cobra.Command) (*sqlite.Store, *workspace.Project, domain.Worktree, error) {
	s, e := openStore()
	if e != nil {
		return nil, nil, domain.Worktree{}, e
	}
	dir, e := os.Getwd()
	if e != nil {
		s.Close()
		return nil, nil, domain.Worktree{}, e
	}
	r, e := workspace.Open(cmd.Context(), dir, s)
	if e != nil {
		s.Close()
		return nil, nil, domain.Worktree{}, e
	}
	w, e := r.Identity(cmd.Context())
	if e != nil {
		s.Close()
	}
	return s, r, w, e
}
func printJSON(cmd *cobra.Command, v any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
func newHookCommand() *cobra.Command {
	hook := &cobra.Command{Use: "hook", Short: "Receive Agent and Git hooks"}
	for _, agent := range []string{"codex", "claude"} {
		hook.AddCommand(&cobra.Command{Use: agent, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			// Capture failures always fail open; stdout is reserved for the hook protocol.
			err := func() error {
				raw, e := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 8<<20))
				if e != nil {
					return e
				}
				logger := logr.FromContextOrDiscard(cmd.Context()).V(1)
				logger.Info("agent hook received", "agent", agent, "input", string(raw))
				s, e := openStore()
				if e != nil {
					return e
				}
				defer s.Close()
				var out map[string]string
				if agent == "claude" {
					out, e = capture.Claude(cmd.Context(), s, raw)
				} else {
					out, e = capture.Codex(cmd.Context(), s, raw)
				}
				if e != nil {
					return e
				}
				logger.Info("agent hook processed", "agent", agent, "response", out)
				if out != nil {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
				}
				return nil
			}()
			if err != nil {
				logr.FromContextOrDiscard(cmd.Context()).Error(err, "agent hook failed", "agent", agent)
				fmt.Fprintf(cmd.ErrOrStderr(), "motif capture: %v\n", err)
			}
			return nil
		}})
	}
	var installedOnly bool
	gitCmd := &cobra.Command{Use: "git <hook-name> [args...]", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "motif capture:", err)
			return nil
		}
		logger := logr.FromContextOrDiscard(cmd.Context()).V(1)
		input := cmd.InOrStdin()
		if logger.Enabled() {
			var raw []byte
			switch args[0] {
			case "post-rewrite", "reference-transaction":
				raw, err = io.ReadAll(input)
				if err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "motif capture:", err)
					return nil
				}
				input = bytes.NewReader(raw)
			}
			logger.Info("Git hook received", "cwd", dir, "args", args, "input", string(raw))
		}
		if installedOnly {
			yes, e := integration.HasGitHook(cmd.Context(), dir, args[0])
			if e != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "motif capture:", e)
				return nil
			}
			if !yes {
				return nil
			}
		}
		s, err := openStore()
		if err == nil {
			defer s.Close()
			err = capture.GitNotification(cmd.Context(), s, dir, args[0], args[1:], input)
		}
		if err != nil {
			logr.FromContextOrDiscard(cmd.Context()).Error(err, "Git hook failed", "args", args)
			fmt.Fprintln(cmd.ErrOrStderr(), "motif capture:", err)
		}
		return nil
	}}
	gitCmd.Flags().BoolVar(&installedOnly, "installed-only", false, "Ignore shared Git hooks in projects without a Motif installation")
	hook.AddCommand(gitCmd)
	return hook
}
func newPatchCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "patch", Short: "Inspect captured changes"}
	var current bool
	var turn, format string
	ls := &cobra.Command{Use: "ls", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if current && turn != "" {
			return &UsageError{fmt.Errorf("--current-turn and --turn are mutually exclusive")}
		}
		if format != "text" && format != "json" {
			return &UsageError{fmt.Errorf("format must be text or json")}
		}
		s, _, w, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		if current {
			turn, e = s.ResolveTurn(w.ID, os.Getenv("CODEX_THREAD_ID"), os.Getenv("CODEX_TOOL_CALL_ID"))
			if e != nil {
				return e
			}
		}
		es, e := s.Events(w.ID)
		if e != nil {
			return e
		}
		if format == "json" {
			filtered := []domain.Event{}
			for _, event := range es {
				if turn == "" || event.TurnID == turn {
					filtered = append(filtered, event)
				}
			}
			return printJSON(cmd, filtered)
		}
		_, e = fmt.Fprint(cmd.OutOrStdout(), capture.PatchList(es, turn))
		return e
	}}
	ls.Flags().BoolVar(&current, "current-turn", false, "Resolve this Codex tool call's turn")
	ls.Flags().StringVar(&turn, "turn", "", "Explicit local turn UID")
	ls.Flags().StringVar(&format, "format", "text", "text or json")
	var getFormat string
	get := &cobra.Command{Use: "get <uid>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if getFormat != "text" && getFormat != "json" {
			return &UsageError{fmt.Errorf("format must be text or json")}
		}
		s, r, w, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		event, e := s.Event(w.ID, args[0])
		if e != nil {
			return e
		}
		b, e := s.Snapshot(event.BeforeID)
		if e != nil {
			return e
		}
		a, e := s.Snapshot(event.AfterID)
		if e != nil {
			return e
		}
		diff, e := r.Diff(cmd.Context(), b.State, a.State)
		if e != nil {
			return e
		}
		if getFormat == "json" {
			return printJSON(cmd, map[string]any{"event": event, "diff": diff})
		}
		if e = printJSON(cmd, event); e != nil {
			return e
		}
		for _, f := range diff {
			fmt.Fprintln(cmd.OutOrStdout(), f.Patch)
		}
		return nil
	}}
	get.Flags().StringVar(&getFormat, "format", "text", "text or json")
	cmd.AddCommand(ls, get)
	return cmd
}
func newBatchCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "patch-batch", Short: "Organize consecutive AI patches by intent"}
	var title, desc string
	create := &cobra.Command{Use: "create <patch-uid>...", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, _, w, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		b, e := s.CreateBatch(cmd.Context(), w.ID, title, desc, args)
		if e != nil {
			return e
		}
		return printJSON(cmd, b)
	}}
	create.Flags().StringVar(&title, "title", "", "Intent title")
	create.Flags().StringVar(&desc, "desc", "", "Intent explanation")
	remove := &cobra.Command{Use: "delete <batch-uid>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, _, w, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		if e = s.DeleteBatch(cmd.Context(), w.ID, args[0]); e != nil {
			return e
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Batch removed (or already absent); original patches are preserved.")
		return nil
	}}
	cmd.AddCommand(create, remove)
	return cmd
}
func checkAgents(agents []string) error {
	for _, a := range agents {
		if a != "codex" && a != "claude" {
			return &UsageError{fmt.Errorf("unsupported agent %q; supports codex, claude", a)}
		}
	}
	return nil
}
func newInstallCommand() *cobra.Command {
	var agents []string
	var yes bool
	cmd := &cobra.Command{Use: "install", Short: "Install project-local Agent and Git hooks", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if e := checkAgents(agents); e != nil {
			return e
		}
		s, r, _, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		plan, e := integration.PlanHooks(cmd.Context(), r)
		if e != nil {
			return e
		}
		if plan.Override {
			fmt.Fprintf(cmd.OutOrStdout(), "WARNING: inherited core.hooksPath = %s\nSet repository-local core.hooksPath to %s. Hooks from the inherited directory (including Git LFS hooks) will no longer run in this repository. Existing hooks in the destination will be preserved and chained. Linked worktrees of this repository share the local setting.\n", plan.Inherited, plan.Path)
		}
		reader := bufio.NewReader(cmd.InOrStdin())
		if len(agents) == 0 {
			if !terminal() {
				return fmt.Errorf("noninteractive install requires --agent codex or --agent claude")
			}
			for _, agent := range []string{"codex", "claude"} {
				_, detectedErr := exec.LookPath(agent)
				detected := detectedErr == nil
				prompt := fmt.Sprintf("[ ] %s (not found; configuration is still available). Install? [y/N] ", agent)
				if detected {
					prompt = fmt.Sprintf("[x] %s. Install? [Y/n] ", agent)
				}
				fmt.Fprint(cmd.OutOrStdout(), prompt)
				line, e := reader.ReadString('\n')
				if e != nil {
					return e
				}
				answer := strings.TrimSpace(line)
				if strings.EqualFold(answer, "y") || (answer == "" && detected) {
					agents = append(agents, agent)
				}
			}
			if len(agents) == 0 {
				return nil
			}
		}
		if plan.Override && !yes {
			if !terminal() {
				return fmt.Errorf("changing inherited hooks requires --yes; no integrations installed")
			}
			fmt.Fprint(cmd.OutOrStdout(), "Use repository-local hooks? [y/N] ")
			line, e := reader.ReadString('\n')
			if e != nil {
				return e
			}
			if !strings.EqualFold(strings.TrimSpace(line), "y") {
				return nil
			}
		}
		binary, e := os.Executable()
		if e != nil {
			return e
		}
		m, added, e := integration.InstallWithOptions(cmd.Context(), r, binary, integration.InstallOptions{ConfirmLocalHooksPath: plan.Override, Agents: agents})
		if e != nil {
			return e
		}
		state := "already configured"
		if added {
			state = "installed"
		}
		for _, agent := range agents {
			path := m.Config.Path
			if agent == "claude" {
				path = m.Claude.Path
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", agent, state, path)
		}
		for _, h := range m.Hooks {
			fmt.Fprintln(cmd.OutOrStdout(), "Git hook:", h.Path)
		}
		for _, agent := range agents {
			diagnostic := integration.DiagnoseAgent(cmd.Context(), r, m, agent)
			fmt.Fprintf(cmd.OutOrStdout(), "%s hooks: %s — %s\n", agent, diagnostic.Status, diagnostic.Detail)
		}
		return nil
	}}
	cmd.Flags().StringArrayVar(&agents, "agent", nil, "Agent to configure (repeatable; codex, claude)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm switching inherited hooks to repository-local hooks")
	return cmd
}
func newUninstallCommand() *cobra.Command {
	var agents []string
	var yes bool
	cmd := &cobra.Command{Use: "uninstall", Short: "Remove project integrations; without --agent, also delete project history", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkAgents(agents); err != nil {
			return err
		}
		s, r, w, err := openProject(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		full := len(agents) == 0
		roots := []string{r.Root}
		var plan sqlite.DeletionPlan
		if full {
			plan, err = s.DeletionPlan(w.RepositoryID)
			if err != nil {
				return err
			}
			roots = plan.Worktrees
		}
		type target struct {
			repo     *workspace.Project
			manifest integration.Manifest
		}
		var targets []target
		// Prepare and validate the complete concrete plan before confirmation or writes.
		for _, root := range roots {
			if _, e := os.Stat(root); os.IsNotExist(e) {
				fmt.Fprintf(cmd.OutOrStdout(), "Missing worktree (history still included in deletion): %s\n", root)
				continue
			}
			repo, e := workspace.Open(cmd.Context(), root, s)
			if e != nil {
				return e
			}
			m, e := integration.Load(repo)
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return e
			}
			removal := integration.RemovalPlan(m, agents)
			if e = integration.Verify(removal); e != nil {
				return e
			}
			targets = append(targets, target{repo, m})
			if removal.Config.Path != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Remove codex config: %s\n%s", removal.Config.Path, removal.Config.Content)
			}
			if removal.Claude != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Remove claude hooks: %s\n%s\n", removal.Claude.Path, removal.Claude.Content)
			}
			for _, h := range removal.Hooks {
				fmt.Fprintf(cmd.OutOrStdout(), "Managed Git hook (retained while another worktree owns it): %s\n%s", h.Path, h.Content)
				if h.Backup != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "Original hook backup:", h.Backup)
				}
			}
		}
		if len(targets) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No managed integration found.")
		}
		if full {
			fmt.Fprintf(cmd.OutOrStdout(), "DELETE all local history for repository %s\nDatabase: %s\n", r.CommonDir, filepath.Join(s.Dir, "data.db"))
			if err = printJSON(cmd, plan); err != nil {
				return err
			}
		}
		if !yes {
			if !terminal() {
				return fmt.Errorf("noninteractive uninstall requires -y; no integrations or history removed")
			}
			fmt.Fprint(cmd.OutOrStdout(), "Proceed? [y/N] ")
			line, e := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if e != nil {
				return e
			}
			if !strings.EqualFold(strings.TrimSpace(line), "y") {
				return nil
			}
		}
		for _, target := range targets {
			if err = integration.UninstallAgents(target.repo, target.manifest, agents); err != nil {
				return err
			}
		}
		if full {
			if err = s.DeleteRepository(cmd.Context(), w.RepositoryID); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Uninstalled.")
		return nil
	}}
	cmd.Flags().StringArrayVar(&agents, "agent", nil, "Remove only specified Agent integrations, preserving history")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Confirm the printed removal plan")
	return cmd
}
func newWebCommand() *cobra.Command {
	var address string
	cmd := &cobra.Command{Use: "web", Short: "Open the local intent review server", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		s, r, w, e := openProject(cmd)
		if e != nil {
			return e
		}
		defer s.Close()
		return web.Serve(cmd.Context(), &review.Service{Repo: r, Worktree: w}, address, cmd.OutOrStdout())
	}}
	cmd.Flags().StringVar(&address, "listen", "127.0.0.1:3939", "Loopback IP and port")
	return cmd
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Inspect installed Agent hooks without executing them", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		s, r, _, err := openProject(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		m, err := integration.Load(r)
		if err != nil {
			return err
		}
		if m.Claude == nil {
			return printJSON(cmd, integration.Diagnose(cmd.Context(), r, m))
		}
		diagnostics := map[string]integration.Diagnostic{"claude": integration.DiagnoseAgent(cmd.Context(), r, m, "claude")}
		if m.Config.Path != "" {
			diagnostics["codex"] = integration.Diagnose(cmd.Context(), r, m)
		}
		return printJSON(cmd, diagnostics)
	}}
}
