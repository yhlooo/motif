package cli_test

import (
	"bytes"
	"encoding/json"
	"github.com/yhlooo/motif/internal/transport/cli"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeBuddyStopFeedbackExitCode(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("MOTIF_DATA_DIR", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	git := func(args ...string) {
		t.Helper()
		if raw, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.com")
	path := filepath.Join(root, "value.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	hook := func(event string, active bool) error {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"cwd": root, "session_id": "s", "hook_event_name": event, "tool_name": "Write", "tool_input": map[string]any{}, "stop_hook_active": active})
		cmd := cli.NewCommand("motif")
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetIn(bytes.NewReader(raw))
		cmd.SetArgs([]string{"hook", "codebuddy"})
		err := cmd.Execute()
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("stdout=%s stderr=%s", &stdout, &stderr)
		}
		return err
	}
	for _, event := range []string{"UserPromptSubmit", "PreToolUse"} {
		if err := hook(event, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := hook("PostToolUse", false); err != nil {
		t.Fatal(err)
	}
	// main prints this error to stderr; it must not be swallowed as a capture failure.
	if err := hook("Stop", false); cli.ExitCode(err) != 2 || !strings.Contains(err.Error(), "patch-batch create") {
		t.Fatalf("%v (exit %d)", err, cli.ExitCode(err))
	}
	if err := hook("Stop", true); err != nil {
		t.Fatal("Stop continuation loop", err)
	}
}

func TestCodeBuddyCLIInstallDoctorAndSelectiveUninstall(t *testing.T) {
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
	if out := run("install", "--agent", "codebuddy", "--yes"); !strings.Contains(out, "codebuddy: installed") {
		t.Fatal(out)
	}
	if out := run("doctor"); !strings.Contains(out, `"codebuddy"`) || !strings.Contains(out, `"configured"`) {
		t.Fatal(out)
	}
	run("install", "--agent", "codex", "--yes")
	run("uninstall", "--agent", "codebuddy", "-y")
	if _, err = os.Stat(filepath.Join(root, ".codebuddy", "settings.local.json")); !os.IsNotExist(err) {
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

func TestCodeBuddyHookFailsOpenAndKeepsStdoutForProtocol(t *testing.T) {
	t.Setenv("MOTIF_DATA_DIR", t.TempDir())
	cmd := cli.NewCommand("motif")
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PreToolUse"}`))
	cmd.SetArgs([]string{"hook", "codebuddy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !strings.Contains(stderr.String(), "motif capture:") {
		t.Fatalf("stdout=%s stderr=%s", &out, &stderr)
	}
}
