package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/app/workspace"
)

func TestMain(m *testing.M) {
	// Do not inherit the developer's hooks, signing settings or aliases.
	_ = os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_ = os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Exit(m.Run())
}

func installProject(t *testing.T) (*workspace.Project, func(...string) string) {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %s %v", args, b, e)
		}
		return strings.TrimSpace(string(b))
	}
	git("init")
	s, e := sqlite.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	r, e := workspace.Open(context.Background(), root, s)
	if e != nil {
		t.Fatal(e)
	}
	return r, git
}

func TestInheritedHooksRequireConfirmationAndRemainUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	r, git := installProject(t)
	git("config", "--global", "core.hooksPath", "~/.git-hooks")
	globalHook := filepath.Join(home, ".git-hooks", "post-commit")
	if e := os.MkdirAll(filepath.Dir(globalHook), 0755); e != nil {
		t.Fatal(e)
	}
	original := "#!/bin/sh\ngit lfs post-commit \"$@\"\n"
	if e := os.WriteFile(globalHook, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	plan, e := integration.PlanHooks(context.Background(), r)
	if e != nil || !plan.Override || plan.Inherited != filepath.Dir(globalHook) {
		t.Fatalf("plan: %+v %v", plan, e)
	}
	if _, _, e = integration.Install(context.Background(), r, "/motif"); e == nil {
		t.Fatal("installed without override confirmation")
	}
	if _, e = os.Stat(filepath.Join(r.Root, ".codex")); !os.IsNotExist(e) {
		t.Fatal("unconfirmed install wrote configuration")
	}
	m, _, e := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{ConfirmLocalHooksPath: true})
	if e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--local", "--get", "core.hooksPath"); got != filepath.Join(r.CommonDir, "hooks") {
		t.Fatal(got)
	}
	for _, h := range m.Hooks {
		if filepath.Dir(h.Path) != filepath.Join(r.CommonDir, "hooks") {
			t.Fatal(h.Path)
		}
	}
	if b, e := os.ReadFile(globalHook); e != nil || string(b) != original {
		t.Fatal("global hook modified", e)
	}
	if _, changed, e := integration.Install(context.Background(), r, "/motif"); e != nil || changed {
		t.Fatal("repeat install", changed, e)
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--get", "core.hooksPath"); got != "~/.git-hooks" {
		t.Fatal("override not removed", got)
	}
}

func TestLocalTildeHooksPathOverridesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	r, git := installProject(t)
	git("config", "--global", "core.hooksPath", "~/global-hooks")
	git("config", "--local", "core.hooksPath", "~/local-hooks")
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range m.Hooks {
		if filepath.Dir(h.Path) != filepath.Join(home, "local-hooks") {
			t.Fatal(h.Path)
		}
	}
	if got := git("config", "--local", "--get", "core.hooksPath"); got != "~/local-hooks" {
		t.Fatal(got)
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--local", "--get", "core.hooksPath"); got != "~/local-hooks" {
		t.Fatal("user choice removed", got)
	}
}

func TestInstallRepairsMissingHooksAndExecutableBit(t *testing.T) {
	r, _ := installProject(t)
	originalPath := filepath.Join(r.CommonDir, "hooks", "post-commit")
	original := "#!/bin/sh\nexit 7\n"
	if e := os.WriteFile(originalPath, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(m.Hooks[0].Path); e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(m.Hooks[1].Path, 0600); e != nil {
		t.Fatal(e)
	}
	if e = integration.Verify(m); e == nil {
		t.Fatal("missing hook reported healthy")
	}
	repaired, changed, e := integration.Install(context.Background(), r, "/motif")
	if e != nil || !changed {
		t.Fatal("repair", e)
	}
	if e = integration.Verify(repaired); e != nil {
		t.Fatal(e)
	}
	if repaired.Hooks[0].Backup != m.Hooks[0].Backup {
		t.Fatal("lost original backup")
	}
	if !strings.HasPrefix(repaired.Hooks[0].Backup, filepath.Join(r.CommonDir, "motif")+string(os.PathSeparator)) {
		t.Fatal("backup outside .git/motif")
	}
	if _, changed, e = integration.Install(context.Background(), r, "/motif"); e != nil || changed {
		t.Fatal("repair not idempotent", e)
	}
	for _, h := range repaired.Hooks {
		if !strings.Contains(h.Content, "# BEGIN MOTIF GIT HOOK\n") || !strings.Contains(h.Content, "# END MOTIF GIT HOOK\n") || !strings.Contains(h.Content, "# https://github.com/yhlooo/motif\n") {
			t.Fatal("missing markers/link")
		}
	}
	if e = integration.Uninstall(r, repaired); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(originalPath); e != nil || string(b) != original {
		t.Fatal("original not restored", e)
	}
}

func TestInstallMigratesLegacyManifestAndChangedHooksPath(t *testing.T) {
	r, git := installProject(t)
	git("config", "core.hooksPath", "old-hooks")
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	// Reproduce both the legacy metadata location and its unmarked wrapper format.
	for i := range m.Hooks {
		h := &m.Hooks[i]
		h.Content = strings.ReplaceAll(h.Content, "# BEGIN MOTIF GIT HOOK\n# https://github.com/yhlooo/motif\n", "# Managed by Motif; see motif-install.json\n")
		h.Content = strings.ReplaceAll(h.Content, "# END MOTIF GIT HOOK\n", "")
		if e = os.WriteFile(h.Path, []byte(h.Content), 0755); e != nil {
			t.Fatal(e)
		}
	}
	raw, _ := json.Marshal(m)
	legacy := filepath.Join(r.GitDir, "motif-install.json")
	if e = os.WriteFile(legacy, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(r.GitDir, "motif", "install.json")); e != nil {
		t.Fatal(e)
	}
	git("config", "--unset", "core.hooksPath")
	migrated, changed, e := integration.Install(context.Background(), r, "/motif")
	if e != nil || !changed {
		t.Fatal("migration", e)
	}
	for _, h := range m.Hooks {
		if _, e = os.Stat(h.Path); !os.IsNotExist(e) {
			t.Fatal("old hook remains", e)
		}
	}
	for _, h := range migrated.Hooks {
		if filepath.Dir(h.Path) != filepath.Join(r.CommonDir, "hooks") {
			t.Fatal(h.Path)
		}
	}
	if _, e = os.Stat(legacy); !os.IsNotExist(e) {
		t.Fatal("legacy manifest remains", e)
	}
	if _, e = os.Stat(filepath.Join(r.GitDir, "motif", "install.json")); e != nil {
		t.Fatal(e)
	}
	if e = integration.Uninstall(r, migrated); e != nil {
		t.Fatal(e)
	}
}

func TestInstallDoesNotOverwriteEditedManagedHook(t *testing.T) {
	r, _ := installProject(t)
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	changed := strings.Replace(m.Hooks[0].Content, "|| true", "|| exit 2", 1)
	if e = os.WriteFile(m.Hooks[0].Path, []byte(changed), 0755); e != nil {
		t.Fatal(e)
	}
	if _, _, e = integration.Install(context.Background(), r, "/motif"); e == nil {
		t.Fatal("edited block overwritten")
	}
	if e = integration.Uninstall(r, m); e == nil {
		t.Fatal("edited block deleted")
	}
	if b, _ := os.ReadFile(m.Hooks[0].Path); string(b) != changed {
		t.Fatal("edited hook changed")
	}
}

func TestWorktreesRetainManagedLocalOverrideUntilLastUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	r, git := installProject(t)
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	git("-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "base")
	linked := filepath.Join(t.TempDir(), "linked")
	git("worktree", "add", "-b", "linked", linked)
	git("config", "--global", "core.hooksPath", filepath.Join(home, "shared"))
	a, _, e := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{ConfirmLocalHooksPath: true})
	if e != nil {
		t.Fatal(e)
	}
	other, e := workspace.Open(context.Background(), linked, r.Store)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := integration.Install(context.Background(), other, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	if b.LocalHooksPath != a.LocalHooksPath {
		t.Fatal("override ownership not inherited")
	}
	if e = integration.Uninstall(r, a); e != nil {
		t.Fatal(e)
	}
	if e = integration.Verify(b); e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--local", "--get", "core.hooksPath"); got != a.LocalHooksPath {
		t.Fatal("override removed too early")
	}
	if e = integration.Uninstall(other, b); e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--get", "core.hooksPath"); got != filepath.Join(home, "shared") {
		t.Fatal("override not restored", got)
	}
}

func TestLegacyBackupMovesUnderMotifDirectory(t *testing.T) {
	r, _ := installProject(t)
	path := filepath.Join(r.CommonDir, "hooks", "post-commit")
	original := "#!/bin/sh\nexit 9\n"
	if e := os.WriteFile(path, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	legacyBackup := path + ".motif-original"
	if e = os.Rename(m.Hooks[0].Backup, legacyBackup); e != nil {
		t.Fatal(e)
	}
	m.Hooks[0].Content = strings.ReplaceAll(m.Hooks[0].Content, m.Hooks[0].Backup, legacyBackup)
	m.Hooks[0].Backup = legacyBackup
	if e = os.WriteFile(path, []byte(m.Hooks[0].Content), 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(m)
	if e = os.WriteFile(filepath.Join(r.GitDir, "motif", "install.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	repaired, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(legacyBackup); !os.IsNotExist(e) {
		t.Fatal("legacy backup remains", e)
	}
	if e = integration.Uninstall(r, repaired); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(path); e != nil || string(b) != original {
		t.Fatal("original not restored", e)
	}
}

func TestInstallRollsBackWhenLocalConfigIsLocked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	r, git := installProject(t)
	git("config", "--global", "core.hooksPath", filepath.Join(home, "global-hooks"))
	hook := filepath.Join(r.CommonDir, "hooks", "post-commit")
	original := "#!/bin/sh\nexit 9\n"
	if e := os.WriteFile(hook, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	// Git will reject the local config write after the hook files have been staged.
	if e := os.WriteFile(filepath.Join(r.CommonDir, "config.lock"), []byte("occupied"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := integration.InstallWithOptions(context.Background(), r, "/motif", integration.InstallOptions{ConfirmLocalHooksPath: true}); e == nil {
		t.Fatal("expected failure")
	}
	if b, e := os.ReadFile(hook); e != nil || string(b) != original {
		t.Fatal("original changed", e)
	}
	if _, e := os.Stat(filepath.Join(r.Root, ".codex", "config.toml")); !os.IsNotExist(e) {
		t.Fatal("config left behind", e)
	}
	if _, e := os.Stat(filepath.Join(r.GitDir, "motif", "install.json")); !os.IsNotExist(e) {
		t.Fatal("manifest left behind", e)
	}
}

func TestInstallPreservesHookSymlinkAndTarget(t *testing.T) {
	r, _ := installProject(t)
	target := filepath.Join(t.TempDir(), "original")
	original := "#!/bin/sh\nexit 4\n"
	if e := os.WriteFile(target, []byte(original), 0755); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(r.CommonDir, "hooks", "post-commit")
	if e := os.Symlink(target, path); e != nil {
		t.Fatal(e)
	}
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	if got, e := os.Readlink(path); e != nil || got != target {
		t.Fatal("symlink not restored", got, e)
	}
	if b, e := os.ReadFile(target); e != nil || string(b) != original {
		t.Fatal("target changed", e)
	}
}

func TestWorktreeHooksPathIsPreserved(t *testing.T) {
	r, git := installProject(t)
	git("config", "extensions.worktreeConfig", "true")
	git("config", "--local", "core.hooksPath", "local-hooks")
	git("config", "--worktree", "core.hooksPath", "worktree-hooks")
	m, _, e := integration.Install(context.Background(), r, "/motif")
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range m.Hooks {
		if filepath.Dir(h.Path) != filepath.Join(r.Root, "worktree-hooks") {
			t.Fatal(h.Path)
		}
	}
	if e = integration.Uninstall(r, m); e != nil {
		t.Fatal(e)
	}
	if got := git("config", "--worktree", "--get", "core.hooksPath"); got != "worktree-hooks" {
		t.Fatal(got)
	}
}
