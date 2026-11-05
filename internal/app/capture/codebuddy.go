package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/workspace"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
	"golang.org/x/sys/unix"
)

type CodeBuddyInput struct {
	ToolHookInput
	GenerationID   string `json:"generation_id"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// CodeBuddy does not guarantee call IDs. Only a single, matching outstanding
// pre/post pair is attributed; overlapping or unmatched calls stay unknown.
type codeBuddySession struct {
	Turn        string `json:"turn"`
	Generation  string `json:"generation,omitempty"`
	Pending     string `json:"pending,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Ambiguous   bool   `json:"ambiguous,omitempty"`
}

func CodeBuddy(ctx context.Context, s *sqlite.Store, raw []byte) (map[string]string, error) {
	var input CodeBuddyInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	switch input.Event {
	case "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "SessionEnd":
	default:
		return nil, nil
	}
	if input.CWD == "" || input.Session == "" {
		return nil, fmt.Errorf("CodeBuddy hook requires cwd and session_id")
	}
	if (input.Event == "PreToolUse" || input.Event == "PostToolUse") && input.Tool == "" {
		return nil, fmt.Errorf("CodeBuddy tool hook requires tool_name")
	}
	r, err := workspace.Open(ctx, input.CWD, s)
	if err != nil {
		return nil, err
	}
	// Adapter state lives with worktree metadata, outside the captured tree.
	dir := filepath.Join(r.GitDir, "motif", "codebuddy")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	release, err := lockCodeBuddy(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer release()
	path := filepath.Join(dir, content.Hash([]byte(input.Session))+".json")
	var state codeBuddySession
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	input.Session = "codebuddy:" + input.Session
	event := input.Event
	if event == "UserPromptSubmit" {
		// Identical prompts are distinct turns; Stop continuations keep this turn.
		state = codeBuddySession{Turn: domain.ID(), Generation: input.GenerationID}
	} else if input.GenerationID != "" && state.Generation != input.GenerationID {
		state = codeBuddySession{Turn: domain.ID(), Generation: input.GenerationID}
	}
	input.Turn = state.Turn
	// Canonical JSON avoids relying on object key order between pre and post.
	var args any
	if len(input.Input) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(input.Input))
		decoder.UseNumber()
		if err = decoder.Decode(&args); err != nil {
			return nil, err
		}
	}
	canonical, _ := json.Marshal(args)
	fingerprint := content.Hash([]byte(input.Tool + "/" + input.ToolID + "/" + string(canonical)))
	switch event {
	case "PreToolUse":
		if state.Pending != "" {
			state.Ambiguous = true
		}
		if state.Turn == "" || state.Ambiguous {
			input.Event = "CodeBuddyObservation"
			state.Pending = ""
		} else {
			state.Pending, state.Fingerprint = domain.ID(), fingerprint
			input.ToolID = state.Pending
		}
	case "PostToolUse":
		if state.Pending == "" || state.Fingerprint != fingerprint || state.Ambiguous {
			input.Event = "CodeBuddyObservation"
			state.Ambiguous = true
		} else {
			input.ToolID = state.Pending
		}
		state.Pending, state.Fingerprint = "", ""
	case "Stop", "SessionEnd":
		state.Pending, state.Fingerprint = "", ""
	}
	if event == "SessionEnd" {
		state = codeBuddySession{}
	}
	// Publish state first: a failed observation must never leave a reusable pair.
	data, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, ".session-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return nil, err
	}
	// A Stop without a known prompt still closes any active observation, but
	// cannot ask for batches belonging to an invented turn.
	if event == "Stop" && input.Turn == "" {
		input.Event = "CodeBuddyObservation"
	}
	out, err := captureAgent(ctx, s, input.ToolHookInput, raw)
	if input.StopHookActive {
		return nil, err
	}
	return out, err
}

func lockCodeBuddy(ctx context.Context, dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, "capture.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
		}
		if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
