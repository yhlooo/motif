package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func turn(tx *gorm.DB, w, session, external string) (string, error) {
	if session == "" || external == "" {
		return "", nil
	}
	row := turnRecord{ID: ID(), WorktreeID: w, Session: session, ExternalID: external}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "worktree_id"}, {Name: "session"}, {Name: "external_id"}}, DoNothing: true}).Create(&row).Error; err != nil {
		return "", err
	}
	row = turnRecord{}
	err := tx.Where("worktree_id = ? AND session = ? AND external_id = ?", w, session, external).Take(&row).Error
	return row.ID, err
}
func call(q *gorm.DB, id string) (Call, error) {
	var row callRecord
	err := q.Where("id = ?", id).Take(&row).Error
	return row.value(), err
}
func appendEvent(tx *gorm.DB, w, segment, base, kind, turnID, callID, pair string, before, after Snapshot) error {
	if Equal(before.State, after.State) {
		return nil
	}
	var latest eventRecord
	err := tx.Where("worktree_id = ?", w).Order("sequence DESC").Take(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	from, until := before.Time, after.Time
	if before.Trigger == "baseline" {
		from = 0
	}
	if kind == "virtual_close" || kind == "virtual_carry" {
		from = 0
		until = 0
	}
	row := eventRecord{ID: ID(), WorktreeID: w, Sequence: latest.Sequence + 1, Kind: kind, Segment: segment, Base: base, TurnID: turnID, CallID: callID, BeforeID: before.ID, AfterID: after.ID, PairID: pair, IntervalStart: from, IntervalEnd: until}
	return tx.Create(&row).Error
}

// Observe atomically publishes a prepared immutable state and advances the cursor.
func (s *Store) Observe(ctx context.Context, o Observation) error {
	prepared, err := s.prepareObservation(ctx, o, nil, nil, nil)
	if err != nil {
		return err
	}
	return s.sourceUpdate(prepared, func(tx *gorm.DB) error { return s.observeTx(tx, o) })
}
func (s *Store) observeTx(tx *gorm.DB, o Observation) error {
	w := o.Worktree.ID
	for _, state := range []State{o.State, o.BaseState, o.CommitState} {
		if err := s.validatePrepared(tx, state); err != nil {
			return err
		}
	}
	if o.Receipt != "" {
		var receipt receiptRecord
		err := tx.Where("worktree_id = ? AND key = ?", w, o.Receipt).Take(&receipt).Error
		if err == nil {
			if receipt.Digest != o.Digest {
				return fmt.Errorf("hook receipt conflict")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	c, err := cursor(tx, w)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if c.Version != o.ExpectedVersion {
		return fmt.Errorf("capture cursor changed; retry observation")
	}
	if o.Trigger == "PostToolUse" {
		if c.ActiveCall == "" {
			return nil
		}
		a, e := call(tx, c.ActiveCall)
		if e != nil {
			return e
		}
		if a.Session != o.Session || a.ExternalID != o.ExternalCall {
			return nil
		}
	}
	turnID, err := turn(tx, w, o.Session, o.Turn)
	if err != nil {
		return err
	}
	reset := c.SnapshotID == "" || o.Reset
	if reset {
		keep := false
		if c.ActiveCall != "" && o.KeepCall {
			a, e := call(tx, c.ActiveCall)
			if e != nil {
				return e
			}
			keep = a.Session == o.CallerSession && a.ExternalID == o.CallerID
		}
		if c.ActiveCall != "" && !keep {
			if err = tx.Model(&callRecord{}).Where("id = ?", c.ActiveCall).Update("status", "incomplete").Error; err != nil {
				return err
			}
		}
		baseline, e := s.saveSnapshot(tx, w, o.Base, "baseline", o.BaseState)
		if e != nil {
			return e
		}
		c.SnapshotID = baseline.ID
		c.Base = o.Base
		if !keep {
			c.ActiveCall = ""
		}
		c.Segment = ID()
	}
	before, err := s.snapshot(tx, c.SnapshotID)
	if err != nil {
		return err
	}
	if err = s.validatePrepared(tx, before.State); err != nil {
		return err
	}
	active := c.ActiveCall
	kind := "unknown"
	eventTurn := turnID
	eventCall := ""
	if active != "" {
		a, e := call(tx, active)
		if e != nil {
			return e
		}
		matched := o.Trigger == "PostToolUse" || ((o.Commit != "" || o.Trigger == "rewrite_observation") && a.Session == o.CallerSession && a.ExternalID == o.CallerID)
		if o.KeepCall && a.Session == o.CallerSession && a.ExternalID == o.CallerID && (o.Trigger == "rewrite_resume" || o.Trigger == "post-checkout") {
			// A Git state transition is not an AI edit; keep only the proven live call.
		} else if matched {
			kind = "tool_edit"
			eventTurn = a.TurnID
			eventCall = active
		} else {
			if e = tx.Model(&callRecord{}).Where("id = ?", active).Update("status", "incomplete").Error; e != nil {
				return e
			}
			c.ActiveCall = ""
		}
	}
	after, err := s.saveSnapshot(tx, w, c.Base, o.Trigger, o.State)
	if err != nil {
		return err
	}
	if o.ResumePair != "" {
		kind = "virtual_carry"
		eventTurn = ""
		eventCall = ""
	}
	if err = appendEvent(tx, w, c.Segment, c.Base, kind, eventTurn, eventCall, o.ResumePair, before, after); err != nil {
		return err
	}
	c.SnapshotID = after.ID
	switch o.Trigger {
	case "PreToolUse":
		if o.Call == nil || turnID == "" || o.ExternalCall == "" {
			return fmt.Errorf("tool call identity is required")
		}
		a := *o.Call
		a.ID = ID()
		a.TurnID = turnID
		a.Session = o.Session
		a.ExternalID = o.ExternalCall
		a.Status = "open"
		row := callRow(a)
		err = tx.Create(&row).Error
		if err != nil {
			return err
		}
		c.ActiveCall = a.ID
	case "PostToolUse":
		if err = tx.Model(&callRecord{}).Where("id = ?", active).Update("status", "finished").Error; err != nil {
			return err
		}
		c.ActiveCall = ""
	}
	if o.Commit != "" {
		tree, e := s.saveSnapshot(tx, w, o.Commit, "commit", o.CommitState)
		if e != nil {
			return e
		}
		pair := ""
		if !Equal(after.State, tree.State) {
			pair = ID()
			if e = appendEvent(tx, w, c.Segment, c.Base, "virtual_close", "", "", pair, after, tree); e != nil {
				return e
			}
		}
		if e = tx.Model(&eventRecord{}).Where("worktree_id = ? AND segment = ? AND final_commit = ?", w, c.Segment, "").Update("final_commit", o.Commit).Error; e != nil {
			return e
		}
		c.Base = o.Commit
		c.Segment = ID()
		if pair != "" {
			if e = appendEvent(tx, w, c.Segment, c.Base, "virtual_carry", "", "", pair, tree, after); e != nil {
				return e
			}
		}
	}
	c.Version++
	row := cursorRecord{WorktreeID: w, SnapshotID: c.SnapshotID, Base: c.Base, ActiveCall: c.ActiveCall, Segment: c.Segment, Version: c.Version}
	if err = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "worktree_id"}}, DoUpdates: clause.AssignmentColumns([]string{"snapshot_id", "base", "active_call", "segment", "version"})}).Create(&row).Error; err != nil {
		return err
	}
	if err = autoUnknown(tx, w); err != nil {
		return err
	}
	if o.Receipt != "" {
		err = tx.Create(&receiptRecord{WorktreeID: w, Key: o.Receipt, Digest: o.Digest}).Error
	}
	return err
}

func (s *Store) ResolveTurn(w, session, external string) (string, error) {
	if session == "" || external == "" {
		return "", fmt.Errorf("current turn is unavailable: run inside a hooked Codex call or use --turn <uid>")
	}
	turns := s.db.Model(&turnRecord{}).Select("id").Where("worktree_id = ?", w)
	var ids []string
	if err := s.db.Model(&callRecord{}).Where("turn_id IN (?) AND session = ? AND external_id = ?", turns, session, external).Pluck("turn_id", &ids).Error; err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", fmt.Errorf("cannot uniquely resolve caller's turn; use --turn <uid>")
	}
	return ids[0], nil
}
func (s *Store) Turn(w, session, external string) (string, error) {
	var row turnRecord
	err := s.db.Where("worktree_id = ? AND session = ? AND external_id = ?", w, session, external).Take(&row).Error
	return row.ID, err
}
