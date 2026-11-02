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
