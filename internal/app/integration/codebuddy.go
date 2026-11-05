package integration

import (
	"path/filepath"

	"github.com/yhlooo/motif/internal/app/workspace"
)

func prepareCodeBuddy(r *workspace.Project, binary string, old *JSONHookConfig) (*JSONHookConfig, []byte, error) {
	groups := map[string]any{}
	for _, event := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "SessionEnd"} {
		groups[event] = map[string]any{"matcher": "", "hooks": []any{map[string]any{
			"type": "command", "command": quote(binary) + " hook codebuddy # motif:codebuddy", "timeout": 30,
		}}}
	}
	// Keep Motif hooks in project-local settings for all CodeBuddy clients.
	return prepareJSONHooks(filepath.Join(r.Root, ".codebuddy", "settings.local.json"), "codebuddy", groups, old)
}
