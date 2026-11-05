package integration_test

import (
	"context"
	"encoding/json"
	"github.com/yhlooo/motif/internal/app/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/integration"
)

func TestClaudeAndCodeBuddyAreActiveWhenConfigurationIsValid(t *testing.T) {
	for _, agent := range []string{"claude", "codebuddy"} {
		t.Run(agent, func(t *testing.T) {
			r, _ := installProject(t)
			ctx := context.Background()
			m, _, err := integration.InstallWithOptions(ctx, r, "/motif", integration.InstallOptions{Agents: []string{agent}})
			if err != nil {
				t.Fatal(err)
			}
			// IDE/plugin users need not have an agent CLI on PATH.
			t.Setenv("PATH", t.TempDir())
			if d := integration.DiagnoseAgent(ctx, r, m, agent); d.Status != "active" {
				t.Fatal(d)
			}
			config := m.Claude
			if agent == "codebuddy" {
				config = m.CodeBuddy
			}
			if err = os.WriteFile(config.Path, []byte(`{"hooks":`), 0600); err != nil {
				t.Fatal(err)
			}
			if d := integration.DiagnoseAgent(ctx, r, m, agent); d.Status != "conflict" {
				t.Fatal("invalid configuration was accepted", d)
			}
		})
	}
}

func TestDiagnoseReportsEffectiveHookTrust(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if _, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	s, err := sqlite.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := workspace.Open(context.Background(), root, s)
	if err != nil {
		t.Fatal(err)
	}
	root = r.Root // Git canonicalizes /var to /private/var on macOS.
	m, _, err := integration.Install(context.Background(), r, "/owned/motif")
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `toml:"command"`
			} `toml:"hooks"`
		} `toml:"hooks"`
	}
	if err = toml.Unmarshal([]byte(m.Config.Content), &config); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, trust := range []string{"trusted", "untrusted", "modified"} {
		t.Run(trust, func(t *testing.T) {
			hooks := []map[string]any{}
			for event, groups := range config.Hooks {
				hooks = append(hooks, map[string]any{"eventName": strings.ToLower(event[:1]) + event[1:], "sourcePath": m.Config.Path, "command": groups[0].Hooks[0].Command, "enabled": true, "trustStatus": trust})
			}
			response, _ := json.Marshal(map[string]any{"id": 2, "result": map[string]any{"data": []any{map[string]any{"cwd": root, "hooks": hooks, "errors": []any{}, "warnings": []any{}}}}})
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
			script := "#!/bin/sh\nread -r init\nprintf '%s\\n' '{\"id\":1,\"result\":{}}'\nread -r initialized\nread -r request\nprintf '%s\\n' " + quote(string(response)) + "\ncat >/dev/null\n"
			if err = os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			diagnostic := integration.Diagnose(context.Background(), r, m)
			expected := "inactive"
			if trust == "trusted" {
				expected = "active"
			}
			if diagnostic.Status != expected || len(diagnostic.Hooks) != 5 {
				t.Fatalf("diagnostic: %+v", diagnostic)
			}
		})
	}
}
