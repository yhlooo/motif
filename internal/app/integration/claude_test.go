package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/integration"
)

func TestClaudeInstallCoexistRepairAndSelectiveRemoval(t *testing.T) {
	r, _ := installProject(t)
	ctx := context.Background()
	config := filepath.Join(r.Root, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(config), 0755); err != nil {
		t.Fatal(err)
	}
	original := `{"permissions":{"allow":["Bash(go test:*)"]},"large":9007199254740993,"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo user-hook"}]}]}}`
	if err := os.WriteFile(config, []byte(original), 0640); err != nil {
		t.Fatal(err)
	}
	install := func(agents ...string) (integration.Manifest, bool) {
		t.Helper()
		m, changed, err := integration.InstallWithOptions(ctx, r, "/tmp/motif with 'quote", integration.InstallOptions{Agents: agents})
		if err != nil {
			t.Fatal(err)
		}
		return m, changed
	}
	m, changed := install("claude", "claude")
	if !changed || m.Config.Path != "" || m.Claude == nil {
		t.Fatalf("%+v %v", m, changed)
	}
	if _, err := os.Stat(filepath.Join(r.Root, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("Claude-only install wrote Codex config", err)
	}
	if err := integration.Verify(m); err != nil {
		t.Fatal(err)
	}
	if _, changed = install("claude"); changed {
		t.Fatal("install not idempotent")
	}
	data, _ := os.ReadFile(config)
	if !strings.Contains(string(data), "9007199254740993") {
		t.Fatal("user numeric setting was rounded")
	}
	info, _ := os.Stat(config)
	if info.Mode().Perm() != 0640 {
		t.Fatal("changed user config permissions")
	}
	m, _ = install("codex")
	if m.Claude == nil {
		t.Fatal("Codex install forgot Claude")
	}
	if err := os.Remove(m.Hooks[0].Path); err != nil {
		t.Fatal(err)
	}
	m, changed = install("claude")
	if !changed {
		t.Fatal("missing Git hook not repaired")
	}
	if err := integration.UninstallAgents(r, m, []string{"claude"}); err != nil {
		t.Fatal(err)
	}
	remaining, err := integration.Load(r)
	if err != nil || remaining.Claude != nil || remaining.Config.Path == "" {
		t.Fatalf("remaining %+v: %v", remaining, err)
	}
	if err = integration.Verify(remaining); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(config)
	var before, after any
	_ = json.Unmarshal([]byte(original), &before)
	_ = json.Unmarshal(data, &after)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatalf("lost unrelated settings: %s", data)
	}
	if err = integration.Uninstall(r, remaining); err != nil {
		t.Fatal(err)
	}
	for _, hook := range m.Hooks {
		if _, err = os.Stat(hook.Path); !os.IsNotExist(err) {
			t.Fatal("last agent left Git hook", err)
		}
	}
}

func TestClaudeConflictPreflightAndUninstall(t *testing.T) {
	r, _ := installProject(t)
	ctx := context.Background()
	m, _, err := integration.InstallWithOptions(ctx, r, "/motif", integration.InstallOptions{Agents: []string{"claude"}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(m.Claude.Path)
	modified := strings.Replace(string(data), "Motif snapshot", "Custom snapshot", 1)
	if modified == string(data) {
		t.Fatal("missing status")
	}
	if err = os.WriteFile(m.Claude.Path, []byte(modified), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = integration.InstallWithOptions(ctx, r, "/motif", integration.InstallOptions{Agents: []string{"codex", "claude"}}); err == nil {
		t.Fatal("overwrote edited hook")
	}
	if _, err = os.Stat(filepath.Join(r.Root, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("preflight failure partially installed Codex")
	}
	if err = integration.Uninstall(r, m); err == nil {
		t.Fatal("uninstalled edited hook")
	}
	actual, _ := os.ReadFile(m.Claude.Path)
	if string(actual) != modified {
		t.Fatal("changed conflicted config")
	}
}

func TestClaudeNewConfigCleanupAndCodexSelectiveRemoval(t *testing.T) {
	r, _ := installProject(t)
	m, _, err := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{Agents: []string{"codex", "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = integration.UninstallAgents(r, m, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	m, err = integration.Load(r)
	if err != nil || m.Config.Path != "" || m.Claude == nil {
		t.Fatalf("%+v %v", m, err)
	}
	if err = integration.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err = integration.Uninstall(r, m); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(m.Claude.Path); !os.IsNotExist(err) {
		t.Fatal("created config left behind", err)
	}
}

func TestClaudeMissingGroupRepairAndUninstallRollback(t *testing.T) {
	r, _ := installProject(t)
	ctx := context.Background()
	m, _, err := integration.InstallWithOptions(ctx, r, "/motif", integration.InstallOptions{Agents: []string{"codex", "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(m.Claude.Path)
	var settings map[string]any
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	delete(settings["hooks"].(map[string]any), "PostToolUse")
	settings["permissions"] = map[string]any{"allow": []any{"Read"}}
	raw, _ = json.Marshal(settings)
	if err = os.WriteFile(m.Claude.Path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	m, changed, err := integration.InstallWithOptions(ctx, r, "/motif", integration.InstallOptions{Agents: []string{"claude"}})
	if err != nil || !changed {
		t.Fatal("missing Claude hook not repaired", changed, err)
	}
	if err = integration.Verify(m); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.Claude.Path)
	// Force failure at the final manifest write after the config was modified.
	manifest := filepath.Join(r.GitDir, "motif", "install.json")
	if err = os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(manifest, 0755); err != nil {
		t.Fatal(err)
	}
	if err = integration.UninstallAgents(r, m, []string{"claude"}); err == nil {
		t.Fatal("expected manifest write failure")
	}
	after, _ := os.ReadFile(m.Claude.Path)
	if string(before) != string(after) {
		t.Fatal("failed uninstall did not restore config")
	}
	if err = integration.Verify(m); err != nil {
		t.Fatal("failed uninstall damaged installation", err)
	}
}
