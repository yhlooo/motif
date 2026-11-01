package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/pkg/gitrepo"
	"github.com/yhlooo/motif/pkg/integration"
	"github.com/yhlooo/motif/pkg/store"
)

func TestInstallPreservesConfigurationAndExistingHooks(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v", b, e)
		}
	}
	git("init")
	git("config", "core.hooksPath", "my hooks")
	if e := os.MkdirAll(filepath.Join(root, "my hooks"), 0755); e != nil {
		t.Fatal(e)
	}
	original := "#!/bin/sh\nprintf 'original\\n'\nexit 7\n"
	hookpath := filepath.Join(root, "my hooks", "post-commit")
	if e := os.WriteFile(hookpath, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	config := filepath.Join(root, ".codex", "config.toml")
	if e := os.MkdirAll(filepath.Dir(config), 0755); e != nil {
		t.Fatal(e)
	}
	userConfig := "model = \"custom\"\n[[hooks.Stop]]\n[[hooks.Stop.hooks]]\ntype = \"command\"\ncommand = \"echo existing\"\n"
	if e := os.WriteFile(config, []byte(userConfig), 0600); e != nil {
		t.Fatal(e)
	}
	binary := filepath.Join(t.TempDir(), "motif's binary")
	if e := os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'motif called\\n'\nexit 1\n"), 0755); e != nil {
		t.Fatal(e)
	}
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	m, added, e := integration.Install(ctx, r, binary)
	if e != nil || !added {
		t.Fatal("install", e)
	}
	_, added, e = integration.Install(ctx, r, binary)
	if e != nil || added {
		t.Fatal("idempotent install", e)
	}
	c := exec.Command(hookpath)
	c.Dir = root
	out, e := c.CombinedOutput()
	if e == nil {
		t.Fatal("existing hook failure swallowed")
	}
	if !strings.Contains(string(out), "original\nmotif called") {
		t.Fatalf("hooks did not both run: %q", out)
	}
	before, e := os.ReadFile(config)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(config, append(before, []byte("\n[features]\ncustom = true\n")...), 0600); e != nil {
		t.Fatal(e)
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	after, e := os.ReadFile(config)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(after), "MOTIF") || !strings.Contains(string(after), userConfig) || !strings.Contains(string(after), "custom = true") {
		t.Fatal("user config was overwritten")
	}
	restored, e := os.ReadFile(hookpath)
	if e != nil || string(restored) != original {
		t.Fatal("existing hook not restored", e)
	}
}
func TestModifiedManagedConfigIsPreserved(t *testing.T) {
	root := t.TempDir()
	c := exec.Command("git", "init")
	c.Dir = root
	if _, e := c.CombinedOutput(); e != nil {
		t.Fatal(e)
	}
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(context.Background(), root, s)
	if e != nil {
		t.Fatal(e)
	}
	m, _, e := integration.Install(context.Background(), r, "/nonexistent/motif")
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(m.Config.Path)
	if e != nil {
		t.Fatal(e)
	}
	changed := strings.Replace(string(data), "timeout = 30", "timeout = 31", 1)
	if e = os.WriteFile(m.Config.Path, []byte(changed), 0600); e != nil {
		t.Fatal(e)
	}
	if e = integration.Uninstall(r, m); e == nil {
		t.Fatal("modified config was removed")
	}
	actual, _ := os.ReadFile(m.Config.Path)
	if string(actual) != changed {
		t.Fatal("conflicting config was overwritten")
	}
}

func TestLinkedWorktreesShareHooksWithoutStacking(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v", b, e)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "t@example.com")
	git("commit", "--allow-empty", "-m", "base")
	otherRoot := filepath.Join(t.TempDir(), "linked")
	git("worktree", "add", "-b", "feature", otherRoot)
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	other, e := gitrepo.Open(ctx, otherRoot, s)
	if e != nil {
		t.Fatal(e)
	}
	m, _, e := integration.Install(ctx, r, "/missing/motif")
	if e != nil {
		t.Fatal(e)
	}
	om, _, e := integration.Install(ctx, other, "/missing/motif")
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range om.Hooks {
		if !h.Shared {
			t.Fatal("shared hook installed twice")
		}
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	if e = integration.Verify(om); e != nil {
		t.Fatal("shared hook removed while still owned", e)
	}
	if e = integration.Uninstall(other, om); e != nil {
		t.Fatal(e)
	}
	for _, h := range om.Hooks {
		if _, e = os.Stat(h.Path); !os.IsNotExist(e) {
			t.Fatal("last owner left managed hook behind", e)
		}
	}
}
