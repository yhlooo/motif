package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/integration"
	"github.com/yhlooo/motif/internal/i18n"
)

func TestDoctorTextShowsHookStateAndNeutralRestartHint(t *testing.T) {
	root := t.TempDir()
	m := integration.Manifest{
		Config:    integration.File{Path: filepath.Join(root, ".codex", "config.toml")},
		CodeBuddy: &integration.JSONHookConfig{File: integration.File{Path: filepath.Join(root, ".codebuddy", "settings.local.json")}},
	}
	diagnostics := map[string]integration.Diagnostic{
		"codex":     {Status: "inactive", Detail: "Check project trust.", Hooks: []integration.HookState{{Event: "PreToolUse", Enabled: true, Trust: "trusted"}, {Event: "Stop", Enabled: false, Trust: "untrusted"}}},
		"codebuddy": {Status: "active"},
	}
	ctx := i18n.ContextWithLocalizer(context.Background(), i18n.NewLocalizer("en"))
	for _, color := range []bool{false, true} {
		var output bytes.Buffer
		if err := writeDoctorResult(ctx, &output, root, m, []string{"codex", "codebuddy"}, diagnostics, color); err != nil {
			t.Fatal(err)
		}
		s := output.String()
		for _, want := range []string{"Hook diagnostics", "Codex", "Check project trust.", "PreToolUse", "enabled", "trusted", "Stop", "disabled", "untrusted", ".codex/config.toml", ".codebuddy/settings.local.json", "CodeBuddy"} {
			if !strings.Contains(s, want) {
				t.Fatalf("missing %q:\n%s", want, s)
			}
		}
		if strings.Contains(s, root) {
			t.Fatal(s)
		}
		if !color && strings.Contains(s, "\x1b[") {
			t.Fatal(s)
		}
		if color && !strings.Contains(s, "CodeBuddy   \x1b[2mConfigured; restart") {
			t.Fatal(s)
		}
	}
}
