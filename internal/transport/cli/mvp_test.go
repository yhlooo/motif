package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/yhlooo/motif/internal/transport/cli"
)

func TestHookStorageAndInputFailuresFailOpen(t *testing.T) {
	for _, input := range []string{"not json", `{"hook_event_name":"Stop","cwd":"/nonexistent","session_id":"s","turn_id":"t"}`} {
		t.Run(input, func(t *testing.T) {
			t.Setenv("MOTIF_DATA_DIR", filepath.Join(t.TempDir(), "data"))
			cmd := cli.NewCommand("motif")
			var out, diagnostic bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&diagnostic)
			cmd.SetIn(bytes.NewBufferString(input))
			cmd.SetArgs([]string{"hook", "codex"})
			if err := cmd.Execute(); err != nil {
				t.Fatal("hook failure blocked development", err)
			}
			if out.Len() != 0 || diagnostic.Len() == 0 {
				t.Fatal("diagnostic leaked into hook protocol or was lost")
			}
		})
	}
}
func TestUsageErrorsHaveExitCodeTwo(t *testing.T) {
	for _, args := range [][]string{{"patch", "get"}, {"install", "--agent", "unsupported"}, {"patch", "ls", "--format", "wrong"}, {"--verbose", "3"}, {"patch", "ls", "--unknown"}} {
		cmd := cli.NewCommand("motif")
		cmd.SetArgs(args)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		if err == nil || cli.ExitCode(err) != 2 {
			t.Fatalf("%v: %v (exit %d)", args, err, cli.ExitCode(err))
		}
	}
}
