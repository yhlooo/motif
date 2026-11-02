package integration_test

import (
	"context"
	"github.com/yhlooo/motif/internal/app/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/integration"
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
	r, e := workspace.Open(ctx, root, s)
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
	r, e := workspace.Open(context.Background(), root, s)
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
	r, e := workspace.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	other, e := workspace.Open(ctx, otherRoot, s)
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

func TestStreamingHooksPreserveArgumentsInputAndOriginalStatus(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	c := exec.Command("git", "init")
	c.Dir = root
	if _, err := c.CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	hookdir := filepath.Join(root, ".git", "hooks")
	originalLog := filepath.Join(t.TempDir(), "original")
	motifLog := filepath.Join(t.TempDir(), "motif")
	shellQuote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	original := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(originalLog) + "\ncat >> " + shellQuote(originalLog) + "\nexit 7\n"
	if err := os.WriteFile(filepath.Join(hookdir, "post-rewrite"), []byte(original), 0755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "motif")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(motifLog) + "\ncat >> " + shellQuote(motifLog) + "\nexit 1\n"
	if err := os.WriteFile(binary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := workspace.Open(ctx, root, s)
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := integration.Install(ctx, r, binary)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(hookdir, "post-rewrite"), "amend", "extra argument")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("old new\nsecond pair\n")
	if _, err = cmd.CombinedOutput(); err == nil {
		t.Fatal("original failure was swallowed")
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 7 {
		t.Fatal(err)
	}
	got, err := os.ReadFile(originalLog)
	if err != nil || string(got) != "amend\nextra argument\nold new\nsecond pair\n" {
		t.Fatalf("original stdin/argv: %q %v", got, err)
	}
	got, err = os.ReadFile(motifLog)
	if err != nil || string(got) != "hook\ngit\npost-rewrite\namend\nextra argument\nold new\nsecond pair\n" {
		t.Fatalf("Motif stdin/argv: %q %v", got, err)
	}
	if err = integration.Uninstall(r, m); err != nil {
		t.Fatal(err)
	}
}
func TestExternalSharedHooksPathKeepsOtherProjectOwner(t *testing.T) {
	ctx := context.Background()
	shared := t.TempDir()
	binary := filepath.Join(t.TempDir(), "motif")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	create := func() *workspace.Project {
		root := t.TempDir()
		for _, args := range [][]string{{"init"}, {"config", "core.hooksPath", shared}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			if raw, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s %v", raw, err)
			}
		}
		r, err := workspace.Open(ctx, root, s)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	a, b := create(), create()
	ma, _, err := integration.Install(ctx, a, binary)
	if err != nil {
		t.Fatal(err)
	}
	mb, _, err := integration.Install(ctx, b, binary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ma.Hooks[0].Content, "--installed-only") {
		t.Fatal("external shared hook would collect uninstalled projects")
	}
	if err = integration.Uninstall(a, ma); err != nil {
		t.Fatal(err)
	}
	if err = integration.Verify(mb); err != nil {
		t.Fatal("removed other project's hook", err)
	}
	if err = integration.Uninstall(b, mb); err != nil {
		t.Fatal(err)
	}
	for _, hook := range mb.Hooks {
		if _, err = os.Stat(hook.Path); !os.IsNotExist(err) {
			t.Fatal("last owner left hook behind", err)
		}
	}
}
