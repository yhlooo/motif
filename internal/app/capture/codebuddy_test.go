package capture_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/app/capture"
)

func buddyHook(t *testing.T, f *fixture, event, tool, args, generation string, active bool) map[string]string {
	t.Helper()
	input := capture.CodeBuddyInput{ToolHookInput: capture.ToolHookInput{Event: event, CWD: f.dir, Session: "session", Tool: tool}, GenerationID: generation, StopHookActive: active}
	if args != "" {
		input.Input = json.RawMessage(args)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	out, err := capture.CodeBuddy(f.ctx, f.s, raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCodeBuddyCaptureWithoutExternalIDs(t *testing.T) {
	f := newFixture(t)
	hook := func(event string, active bool) map[string]string {
		return buddyHook(t, f, event, "Edit", `{"description":"Buddy edit"}`, "", active)
	}
	hook("UserPromptSubmit", false)
	f.write("outside.txt", "external\n")
	hook("PreToolUse", false)
	f.write("value.txt", "20\n")
	hook("PostToolUse", false)
	// The same arguments on the next call must create a distinct patch.
	hook("PreToolUse", false)
	f.write("value.txt", "30\n")
	hook("PostToolUse", false)
	es := f.events()
	if len(es) != 3 || es[0].Kind != "unknown" || es[1].Kind != "tool_edit" || es[2].Kind != "tool_edit" {
		t.Fatalf("%+v", es)
	}
	if es[1].Call.Session != "codebuddy:session" || es[1].Call.Reason != "Buddy edit" || es[1].CallID == es[2].CallID || es[1].TurnID != es[2].TurnID {
		t.Fatalf("%+v", es)
	}
	if out := hook("Stop", false); out["decision"] != "block" || !strings.Contains(out["reason"], es[1].ID) {
		t.Fatal(out)
	}
	if out := hook("Stop", true); out != nil {
		t.Fatal("continuation loop", out)
	}
	if _, err := f.s.CreateBatch(f.ctx, f.w.ID, "Buddy", "two edits", []string{es[1].ID, es[2].ID}); err != nil {
		t.Fatal(err)
	}
	if out := hook("Stop", false); out != nil {
		t.Fatal(out)
	}
	hook("UserPromptSubmit", false)
	hook("PreToolUse", false)
	f.write("value.txt", "40\n")
	hook("PostToolUse", false)
	es = f.events()
	if es[2].TurnID == es[3].TurnID {
		t.Fatal("successive prompts share a turn")
	}
	hook("SessionEnd", false)
}

func TestCodeBuddyAmbiguousAndMissingEventsStayUnknown(t *testing.T) {
	for _, scenario := range []string{"overlap", "mismatch", "missing-pre", "missing-prompt", "missing-post"} {
		t.Run(scenario, func(t *testing.T) {
			f := newFixture(t)
			hook := func(event, args string) { buddyHook(t, f, event, "Edit", args, "", false) }
			if scenario != "missing-prompt" {
				hook("UserPromptSubmit", "{}")
			}
			if scenario != "missing-pre" {
				hook("PreToolUse", "{}")
			}
			if scenario == "overlap" {
				hook("PreToolUse", "{}")
			}
			f.write("value.txt", "20\n")
			switch scenario {
			case "mismatch":
				hook("PostToolUse", `{"other":true}`)
			case "missing-post":
				hook("Stop", "{}")
			default:
				hook("PostToolUse", "{}")
			}
			for _, e := range f.events() {
				if e.Kind != "unknown" {
					t.Fatalf("ambiguous edit attributed: %+v", e)
				}
			}
			if len(f.events()) != 1 {
				t.Fatal(f.events())
			}
			// A new user prompt restores pairing after an ambiguous turn.
			hook("UserPromptSubmit", "{}")
			hook("PreToolUse", "{}")
			f.write("value.txt", "30\n")
			hook("PostToolUse", "{}")
			if es := f.events(); len(es) != 2 || es[1].Kind != "tool_edit" {
				t.Fatal(es)
			}
		})
	}
}

func TestCodeBuddyGenerationAndCanonicalArguments(t *testing.T) {
	f := newFixture(t)
	buddyHook(t, f, "UserPromptSubmit", "", "", "g1", false)
	buddyHook(t, f, "PreToolUse", "Write", `{"b":1,"a":9007199254740993}`, "g1", false)
	f.write("value.txt", "20\n")
	buddyHook(t, f, "PostToolUse", "Write", `{"a":9007199254740993,"b":1}`, "g1", false)
	buddyHook(t, f, "PreToolUse", "Write", `{}`, "g2", false)
	f.write("value.txt", "30\n")
	buddyHook(t, f, "PostToolUse", "Write", `{}`, "g2", false)
	es := f.events()
	if len(es) != 2 || es[0].Kind != "tool_edit" || es[1].Kind != "tool_edit" || es[0].TurnID == es[1].TurnID {
		t.Fatal(es)
	}
}

func TestCodeBuddyRejectsInvalidIdentity(t *testing.T) {
	f := newFixture(t)
	for _, raw := range []string{`{`, `{"hook_event_name":"Stop"}`, `{"hook_event_name":"PreToolUse","session_id":"s"}`} {
		if _, err := capture.CodeBuddy(f.ctx, f.s, []byte(raw)); err == nil {
			t.Fatal("accepted invalid hook", raw)
		}
	}
	if len(f.events()) != 0 {
		t.Fatal(f.events())
	}
}
