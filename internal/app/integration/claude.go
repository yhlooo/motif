package integration

import (
	"context"
	"path/filepath"

	"github.com/yhlooo/motif/internal/app/workspace"
	"github.com/yhlooo/motif/internal/i18n"
)

type ClaudeConfig = JSONHookConfig

func claudeGroups(ctx context.Context, binary string) map[string]any {
	groups := map[string]any{}
	for _, hook := range codexHooks {
		if hook.event == "Interrupt" {
			continue
		} // Claude has no Interrupt hook.
		groups[hook.event] = map[string]any{"hooks": []any{map[string]any{
			"type": "command", "command": quote(binary) + " hook claude || true # motif:claude",
			"timeout": 30, "async": false, "statusMessage": i18n.TContext(ctx, hook.statusMessage),
		}}}
	}
	// Failed commands can still have changed files; close the matching call too.
	groups["PostToolUseFailure"] = groups["PostToolUse"]
	groups["StopFailure"] = groups["SessionEnd"]
	return groups
}

func prepareClaude(ctx context.Context, r *workspace.Project, binary string, old *ClaudeConfig) (*ClaudeConfig, []byte, error) {
	return prepareJSONHooks(filepath.Join(r.Root, ".claude", "settings.local.json"), "claude", claudeGroups(ctx, binary), old)
}

func removeClaude(config *ClaudeConfig, allowMissing bool) ([]byte, error) {
	return removeJSONHooks(config, allowMissing, "claude")
}
