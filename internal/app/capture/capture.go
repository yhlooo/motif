// Package capture adapts hooks to the source-independent state/event store.
package capture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"io"
	"os"
	"strings"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/domain"
)

type CodexInput struct {
	Event   string          `json:"hook_event_name"`
	CWD     string          `json:"cwd"`
	Session string          `json:"session_id"`
	Turn    string          `json:"turn_id"`
	ToolID  string          `json:"tool_use_id"`
	Tool    string          `json:"tool_name"`
	Model   string          `json:"model"`
	Agent   string          `json:"agent_id"`
	Input   json.RawMessage `json:"tool_input"`
}

func Observe(ctx context.Context, r *workspace.Project, w domain.Worktree, o domain.Observation) error {
	c, err := r.Store.Cursor(w.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	o.ExpectedVersion = c.Version
	head, err := r.Head(ctx)
	if err != nil {
		return err
	}
	o.Worktree = w
	o.Base = head
	if o.Commit != "" {
		o.Commit = head
		raw, e := r.Git(ctx, "rev-list", "--parents", "-n", "1", head)
		if e != nil {
			return e
		}
		fields := strings.Fields(string(raw))
		if len(fields) >= 2 {
			o.Parent = fields[1]
		}
		if c.SnapshotID != "" && c.Base == head {
			return nil
		} // Repeated post-commit notification.
		if c.SnapshotID == "" {
			o.Base = o.Parent
		} else if c.Base != o.Parent {
			return nil // amend/rebase association is handled by post-rewrite.
		} else {
			o.Base = c.Base
		}
		o.CommitState, err = r.Tree(ctx, head)
		if err != nil {
			return err
		}
	} else if c.SnapshotID != "" && c.Base != head {
		o.Reset = true
	}
	o.BaseState, err = r.Tree(ctx, o.Base)
	if err != nil {
		return err
	}
	var previous domain.State
	if c.SnapshotID != "" && !o.Reset {
		sn, e := r.Store.Snapshot(c.SnapshotID)
		if e != nil {
			return e
		}
		previous = sn.State
	}
	o.State, err = r.Worktree(ctx, o.BaseState, previous)
	if err != nil {
		return err
	}
	return r.Store.Observe(ctx, o)
}
func Codex(ctx context.Context, s *store.Store, raw []byte) (map[string]string, error) {
	var input CodexInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	switch input.Event {
	case "PreToolUse", "PostToolUse", "Stop", "Interrupt", "SessionEnd":
	default:
		return nil, nil
	}
	if input.CWD == "" || input.Session == "" {
		return nil, fmt.Errorf("hook requires cwd and session_id")
	}
	if (input.Event == "PreToolUse" || input.Event == "PostToolUse" || input.Event == "Stop") && input.Turn == "" {
		return nil, fmt.Errorf("hook requires turn_id; check Codex hook support")
	}
	if (input.Event == "PreToolUse" || input.Event == "PostToolUse") && input.ToolID == "" {
		return nil, fmt.Errorf("hook requires tool_use_id")
	}
	r, err := workspace.Open(ctx, input.CWD, s)
	if err != nil {
		return nil, err
	}
	w, err := r.Identity(ctx)
	if err != nil {
		return nil, err
	}
	o := domain.Observation{Trigger: input.Event, Session: input.Session, Turn: input.Turn, ExternalCall: input.ToolID}
	if input.Event == "PreToolUse" || input.Event == "PostToolUse" {
		o.Receipt = input.Session + "/" + input.Turn + "/" + input.ToolID + "/" + input.Event
		h := sha256.Sum256(raw)
		o.Digest = hex.EncodeToString(h[:])
	}
	if input.Event == "PreToolUse" {
		a := &domain.Call{Tool: input.Tool, Model: input.Model, Agent: input.Agent, Input: input.Input, UserName: r.Config(ctx, "user.name"), UserEmail: r.Config(ctx, "user.email")}
		var toolInput map[string]json.RawMessage
		if json.Unmarshal(input.Input, &toolInput) == nil {
			for _, key := range []string{"justification", "description"} {
				var reason string
				if json.Unmarshal(toolInput[key], &reason) == nil && reason != "" {
					a.Reason = reason
					a.ReasonSource = "tool_input." + key
					break
				}
			}
		}
		o.Call = a
	}
	if err = Observe(ctx, r, w, o); err != nil {
		return nil, err
	}
	if input.Event != "Stop" {
		return nil, nil
	}
	turn, err := s.Turn(w.ID, input.Session, input.Turn)
	if err != nil {
		return nil, err
	}
	es, err := s.Events(w.ID)
	if err != nil {
		return nil, err
	}
	var pending []domain.Event
	for _, e := range es {
		if !e.Projected && e.TurnID == turn && e.Kind == "tool_edit" && e.BatchID == "" {
			pending = append(pending, e)
		}
	}
	if len(pending) == 0 {
		return nil, nil
	}
	listing := PatchList(es, turn)
	return map[string]string{"decision": "block", "reason": "Motif: 请先完成本轮 AI 修改分批，再结束。\n" + listing + "\n查看: motif patch get <uid>\n创建: motif patch-batch create --title='标题' --desc='真实修改原因' <uid1> <uid2> ...\n每批只能包含同轮、同工作区、同提交边界内连续的 AI patch。不要把未知或虚拟项加入 AI 批次。"}, nil
}
func PatchList(events []domain.Event, turn string) string {
	var b strings.Builder
	for _, e := range events {
		if turn != "" && e.TurnID != turn {
			continue
		}
		reason := "unknown"
		if e.Call != nil && e.Call.Reason != "" {
			reason = e.Call.Reason
		}
		commit := e.Commit
		if commit == "" {
			commit = "uncommitted"
		}
		fmt.Fprintf(&b, "%s  %s  turn=%s  commit=%s  observed=%d..%d\n  reason: %s\n", e.ID, e.Kind, e.TurnID, commit, e.From, e.Until, reason)
		if e.BatchID != "" {
			fmt.Fprintf(&b, "  batch: %s %s\n  description: %s\n", e.BatchID, e.BatchTitle, e.BatchDescription)
		}
	}
	return b.String()
}
func Git(ctx context.Context, s *store.Store, dir, event string) error {
	return GitNotification(ctx, s, dir, event, nil, strings.NewReader(""))
}
func GitNotification(ctx context.Context, s *store.Store, dir, event string, args []string, input io.Reader) error {
	r, err := workspace.Open(ctx, dir, s)
	if err != nil {
		return err
	}
	w, err := r.Identity(ctx)
	if err != nil {
		return err
	}
	o := domain.Observation{Trigger: event, CallerSession: os.Getenv("CODEX_THREAD_ID"), CallerID: os.Getenv("CODEX_TOOL_CALL_ID")}
	switch event {
	case "post-commit":
		o.Commit = "HEAD"
	case "post-checkout":
		o.KeepCall = true
	case "post-rewrite":
		if len(args) != 1 {
			return fmt.Errorf("post-rewrite requires amend or rebase")
		}
		return Rewrite(ctx, r, w, args[0], input)
	case "reference-transaction":
		if len(args) != 1 {
			return fmt.Errorf("reference-transaction requires phase")
		}
		return References(ctx, r, w, args[0], input)
	default:
		return fmt.Errorf("unsupported Git observation: %s", event)
	}
	return Observe(ctx, r, w, o)
}
