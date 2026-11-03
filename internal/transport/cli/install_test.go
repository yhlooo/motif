package cli_test

import (
	"bytes"
	"github.com/yhlooo/motif/internal/transport/cli"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWarnsBeforeRequiringLocalOverrideConfirmation(t *testing.T) {
	for _, yes := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconfirmed", true: "confirmed"}[yes], func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
			t.Setenv("MOTIF_DATA_DIR", filepath.Join(t.TempDir(), "data"))
			git := func(args ...string) string {
				t.Helper()
				b, e := exec.Command("git", args...).CombinedOutput()
				if e != nil {
					t.Fatalf("git %v: %s %v", args, b, e)
				}
				return strings.TrimSpace(string(b))
			}
			git("init")
			git("config", "--global", "core.hooksPath", filepath.Join(t.TempDir(), "global-hooks"))
			// Keep installed Codex binaries out of this isolated CLI test.
			gitBinary, e := exec.LookPath("git")
			if e != nil {
				t.Fatal(e)
			}
			bin := t.TempDir()
			if e = os.Symlink(gitBinary, filepath.Join(bin, "git")); e != nil {
				t.Fatal(e)
			}
			t.Setenv("PATH", bin)
			cmd := cli.NewCommand("motif")
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			args := []string{"install", "--agent", "codex"}
			if yes {
				args = append(args, "--yes")
			}
			cmd.SetArgs(args)
			e = cmd.Execute()
			if !strings.Contains(out.String(), "WARNING:") || !strings.Contains(out.String(), "Git LFS") {
				t.Fatal("missing warning", out.String())
			}
			if !yes {
				if e == nil || !strings.Contains(e.Error(), "--yes") {
					t.Fatal(e)
				}
				if _, e = os.Stat(filepath.Join(root, ".codex", "config.toml")); !os.IsNotExist(e) {
					t.Fatal("wrote without confirmation", e)
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				if got := git("config", "--local", "--get", "core.hooksPath"); !filepath.IsAbs(got) || filepath.Base(got) != "hooks" {
					t.Fatal(got)
				}
			}
		})
	}
}

func TestClaudeCLIInstallDoctorAndSelectiveUninstall(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("MOTIF_DATA_DIR", t.TempDir())
	if raw, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	// Do not invoke installed agent binaries during the integration lifecycle test.
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err = os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	run := func(args ...string) string {
		t.Helper()
		cmd := cli.NewCommand("motif")
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(args, err, out.String())
		}
		return out.String()
	}
	if out := run("install", "--agent", "claude"); !strings.Contains(out, "claude: installed") {
		t.Fatal(out)
	}
	if out := run("doctor"); !strings.Contains(out, `"claude"`) || !strings.Contains(out, `"unavailable"`) {
		t.Fatal(out)
	}
	run("install", "--agent", "codex")
	run("uninstall", "--agent", "claude", "-y")
	if _, err = os.Stat(filepath.Join(root, ".claude", "settings.local.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, ".codex", "config.toml")); err != nil {
		t.Fatal(err)
	}
	run("uninstall", "--agent", "codex", "-y")
	if _, err = os.Stat(filepath.Join(root, ".git", "hooks", "post-commit")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestClaudeHookFailsOpenAndKeepsStdoutForProtocol(t *testing.T) {
	t.Setenv("MOTIF_DATA_DIR", t.TempDir())
	cmd := cli.NewCommand("motif")
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PreToolUse"}`))
	cmd.SetArgs([]string{"hook", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !strings.Contains(stderr.String(), "motif capture:") {
		t.Fatalf("stdout=%s stderr=%s", &out, &stderr)
	}
}
