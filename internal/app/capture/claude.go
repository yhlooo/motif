package capture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
)

// ClaudeInput uses prompt_id rather than Codex's turn_id. Session identities
// are namespaced before crossing into the source-independent capture service.
type ClaudeInput struct {
	ToolHookInput
	PromptID       string `json:"prompt_id"`
	StopHookActive bool   `json:"stop_hook_active"`
}

func Claude(ctx context.Context, s *sqlite.Store, raw []byte) (map[string]string, error) {
	var input ClaudeInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	switch input.Event {
	case "PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "StopFailure", "SessionEnd":
	default:
		return nil, nil
	}
	if input.CWD == "" || input.Session == "" {
		return nil, fmt.Errorf("Claude hook requires cwd and session_id")
	}
	if input.Event != "SessionEnd" && input.PromptID == "" {
		return nil, fmt.Errorf("Claude hook requires prompt_id; update Claude Code to a version providing prompt identities")
	}
	if (input.Event == "PreToolUse" || input.Event == "PostToolUse" || input.Event == "PostToolUseFailure") && input.ToolID == "" {
		return nil, fmt.Errorf("Claude hook requires tool_use_id")
	}
	input.Session = "claude:" + input.Session
	input.Turn = input.PromptID
	if input.Event == "PostToolUseFailure" {
		input.Event = "PostToolUse"
	}
	if input.Event == "StopFailure" {
		input.Event = "Interrupt"
	}
	out, err := captureAgent(ctx, s, input.ToolHookInput, raw)
	// Still observe repeated Stop hooks, but avoid an endless continuation loop.
	if input.StopHookActive {
		return nil, err
	}
	return out, err
}
