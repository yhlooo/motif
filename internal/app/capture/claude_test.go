package capture_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/capture"
)

func TestClaudeCaptureFailureBatchStopAndPromptIsolation(t *testing.T) {
	f := newFixture(t)
	hook := func(event, call, prompt string, active bool) map[string]string {
		t.Helper()
		input := capture.ClaudeInput{ToolHookInput: capture.ToolHookInput{Event: event, CWD: f.dir, Session: "session", ToolID: call, Tool: "Edit", Input: json.RawMessage(`{"description":"Claude edit"}`)}, PromptID: prompt, StopHookActive: active}
		raw, _ := json.Marshal(input)
		out, err := capture.Claude(f.ctx, f.s, raw)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	f.write("outside.txt", "external\n")
	hook("PreToolUse", "one", "prompt-1", false)
	hook("PreToolUse", "one", "prompt-1", false)
	f.write("value.txt", "30\n")
	hook("PostToolUseFailure", "one", "prompt-1", false)
	hook("PostToolUseFailure", "one", "prompt-1", false)
	es := f.events()
	if len(es) != 2 || es[0].Kind != "unknown" || es[1].Kind != "tool_edit" || es[1].Call.Reason != "Claude edit" {
		t.Fatalf("%+v", es)
	}
	if es[1].Call.Session != "claude:session" {
		t.Fatal("session not namespaced")
	}
	out := hook("Stop", "", "prompt-1", false)
	if out["decision"] != "block" || !strings.Contains(out["reason"], es[1].ID) {
		t.Fatalf("%+v", out)
	}
	if out = hook("Stop", "", "prompt-1", true); out != nil {
		t.Fatal("Stop continuation loop", out)
	}
	if _, err := f.s.CreateBatch(f.ctx, f.w.ID, "fix", "Claude change", []string{es[1].ID}); err != nil {
		t.Fatal(err)
	}
	if out = hook("Stop", "", "prompt-1", false); out != nil {
		t.Fatal("batched edit still blocks", out)
	}
	hook("PreToolUse", "two", "prompt-2", false)
	f.write("value.txt", "40\n")
	hook("PostToolUse", "two", "prompt-2", false)
	es = f.events()
	if es[1].TurnID == es[2].TurnID {
		t.Fatal("different prompts share a turn")
	}
	// The same raw session/call names on Codex must not collide with Claude.
	f.hook("PreToolUse", "two", "prompt-2")
	f.write("value.txt", "50\n")
	f.hook("PostToolUse", "two", "prompt-2")
	es = f.events()
	if es[2].TurnID == es[3].TurnID {
		t.Fatal("Codex/Claude turn collision")
	}
	hook("SessionEnd", "", "", false)
}

func TestClaudeRejectsMissingIdentityWithoutAttributingChanges(t *testing.T) {
	f := newFixture(t)
	for _, input := range []string{
		`{"hook_event_name":"PreToolUse","session_id":"s","tool_use_id":"t"}`,
		`{"hook_event_name":"PreToolUse","session_id":"s","prompt_id":"p"}`,
		`{"hook_event_name":"Stop","session_id":"s"}`,
	} {
		var payload map[string]any
		_ = json.Unmarshal([]byte(input), &payload)
		payload["cwd"] = f.dir
		raw, _ := json.Marshal(payload)
		if _, err := capture.Claude(f.ctx, f.s, raw); err == nil {
			t.Fatal("accepted missing identity", input)
		}
	}
	if len(f.events()) != 0 {
		t.Fatal("invalid hooks created changes")
	}
}
