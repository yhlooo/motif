package domain

import "fmt"

// CaptureInput contains every fact needed by the state machine. No IO, database
// transaction, or context-dependent lookup is available to the planner.
type CaptureInput struct {
	Observation Observation
	Cursor      Cursor
	Before      Snapshot
	ActiveCall  *Call
	TurnID      string
	Pending     []string
}
type CallTransition struct{ ID, Status string }
type ChangeCommit struct{ ChangeID, Commit string }
type CapturePlan struct {
	ExpectedVersion int64
	Worktree        Worktree
	Cursor          Cursor
	Observation     Observation
	Snapshots       []Snapshot
	Changes         []Event
	Associations    []ChangeCommit
	Call            *Call
	Calls           []CallTransition
	Ignored         bool
}

func PlanCapture(in CaptureInput) (CapturePlan, error) {
	o, c := in.Observation, in.Cursor
	p := CapturePlan{ExpectedVersion: c.Version, Worktree: o.Worktree, Observation: o}
	if c.Version != o.ExpectedVersion {
		return p, fmt.Errorf("capture cursor changed; retry observation")
	}
	a := in.ActiveCall
	if o.Trigger == "PostToolUse" && (a == nil || a.Session != o.Session || a.ExternalID != o.ExternalCall) {
		p.Ignored = true
		return p, nil
	}
	snapshot := func(base, trigger string, state State) Snapshot {
		sn := Snapshot{ID: ID(), WorktreeID: o.Worktree.ID, Base: base, Trigger: trigger, Time: Now(), State: state}
		p.Snapshots = append(p.Snapshots, sn)
		return sn
	}
	finish := func(id, status string) {
		if id != "" {
			p.Calls = append(p.Calls, CallTransition{id, status})
		}
	}
	before := in.Before
	if c.SnapshotID == "" || o.Reset {
		keep := a != nil && o.KeepCall && a.Session == o.CallerSession && a.ExternalID == o.CallerID
		if !keep {
			finish(c.ActiveCall, "incomplete")
			c.ActiveCall = ""
			a = nil
		}
		before = snapshot(o.Base, "baseline", o.BaseState)
		c.SnapshotID = before.ID
		c.Base = o.Base
		c.Segment = ID()
	}
	kind, turnID, callID := "unknown", in.TurnID, ""
	active := c.ActiveCall
	if a != nil {
		matched := o.Trigger == "PostToolUse" || ((o.Commit != "" || o.Trigger == "rewrite_observation") && a.Session == o.CallerSession && a.ExternalID == o.CallerID)
		keep := o.KeepCall && a.Session == o.CallerSession && a.ExternalID == o.CallerID && (o.Trigger == "rewrite_resume" || o.Trigger == "post-checkout")
		if !keep {
			if matched {
				kind = "tool_edit"
				turnID = a.TurnID
				callID = a.ID
			} else {
				finish(active, "incomplete")
				c.ActiveCall = ""
			}
		}
	}
	appendChange := func(kind, turn, call, pair string, before, after Snapshot) string {
		if Equal(before.State, after.State) {
			return ""
		}
		from, until := before.Time, after.Time
		if before.Trigger == "baseline" {
			from = 0
		}
		if kind == "virtual_close" || kind == "virtual_carry" {
			from, until = 0, 0
		}
		e := Event{ID: ID(), WorktreeID: o.Worktree.ID, Kind: kind, Segment: c.Segment, Base: c.Base, TurnID: turn, CallID: call, BeforeID: before.ID, AfterID: after.ID, PairID: pair, From: from, Until: until}
		p.Changes = append(p.Changes, e)
		return e.ID
	}
	after := snapshot(c.Base, o.Trigger, o.State)
	if o.ResumePair != "" {
		kind = "virtual_carry"
		turnID = ""
		callID = ""
	}
	appendChange(kind, turnID, callID, o.ResumePair, before, after)
	c.SnapshotID = after.ID
	switch o.Trigger {
	case "PreToolUse":
		if o.Call == nil || in.TurnID == "" || o.ExternalCall == "" {
			return p, fmt.Errorf("tool call identity is required")
		}
		call := *o.Call
		call.ID = ID()
		call.TurnID = in.TurnID
		call.Session = o.Session
		call.ExternalID = o.ExternalCall
		call.Status = "open"
		p.Call = &call
		c.ActiveCall = call.ID
	case "PostToolUse":
		finish(active, "finished")
		c.ActiveCall = ""
	}
	if o.Commit != "" {
		tree := snapshot(o.Commit, "commit", o.CommitState)
		pair := ""
		if !Equal(after.State, tree.State) {
			pair = ID()
			appendChange("virtual_close", "", "", pair, after, tree)
		}
		for _, id := range in.Pending {
			p.Associations = append(p.Associations, ChangeCommit{id, o.Commit})
		}
		for _, e := range p.Changes {
			p.Associations = append(p.Associations, ChangeCommit{e.ID, o.Commit})
		}
		c.Base = o.Commit
		c.Segment = ID()
		if pair != "" {
			appendChange("virtual_carry", "", "", pair, tree, after)
		}
	}
	c.Version++
	p.Cursor = c
	return p, nil
}
