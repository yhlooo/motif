package sqlite

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func call(q *gorm.DB, id string) (domain.Call, error) {
	var r callRecord
	e := q.Where("id = ?", id).Take(&r).Error
	return r.value(), e
}

// PrepareCapture reads a consistent metadata view, resolves immutable snapshots
// outside that transaction, and runs the IO-free domain planner.
func (s *Store) PrepareCapture(ctx context.Context, o domain.Observation) (plan domain.CapturePlan, err error) {
	in := domain.CaptureInput{Observation: o}
	duplicate := false
	err = s.update(ctx, func(tx *gorm.DB) error {
		if o.Receipt != "" {
			var r receiptRecord
			e := tx.Where("worktree_id = ? AND key = ?", o.Worktree.ID, o.Receipt).Take(&r).Error
			if e == nil {
				if r.Digest != o.Digest {
					return fmt.Errorf("hook receipt conflict")
				}
				duplicate = true
				return nil
			}
			if !errors.Is(e, ErrNotFound) {
				return e
			}
		}
		c, e := cursor(tx, o.Worktree.ID)
		if e != nil && !errors.Is(e, ErrNotFound) {
			return e
		}
		in.Cursor = c
		if c.Version != o.ExpectedVersion {
			return domain.ErrCaptureConflict
		}
		if c.ActiveCall != "" {
			a, e := call(tx, c.ActiveCall)
			if e != nil {
				return e
			}
			in.ActiveCall = &a
		}
		if o.Session != "" && o.Turn != "" {
			var turn turnRecord
			e := tx.Where("worktree_id = ? AND session = ? AND external_id = ?", o.Worktree.ID, o.Session, o.Turn).Take(&turn).Error
			if e == nil {
				in.TurnID = turn.ID
			} else if errors.Is(e, ErrNotFound) {
				in.TurnID = ContentHash([]byte(o.Worktree.ID + "/" + o.Session + "/" + o.Turn))
			} else {
				return e
			}
		}
		if o.Commit != "" && c.Segment != "" {
			return tx.Model(&eventRecord{}).Where("worktree_id = ? AND segment = ?", o.Worktree.ID, c.Segment).Pluck("id", &in.Pending).Error
		}
		return nil
	})
	if err != nil {
		return plan, err
	}
	if duplicate {
		return domain.CapturePlan{Ignored: true}, nil
	}
	if in.Cursor.SnapshotID != "" && !o.Reset {
		in.Before, err = s.SnapshotContext(ctx, in.Cursor.SnapshotID)
		if err != nil {
			return plan, err
		}
	}
	return domain.PlanCapture(in)
}

func (s *Store) Observe(ctx context.Context, o domain.Observation) error {
	release, e := (content.Objects{Dir: s.Dir}).LeaseContext(ctx, false)
	if e != nil {
		return e
	}
	defer release()
	stage := time.Now()
	plan, e := s.PrepareCapture(ctx, o)
	if e != nil {
		return e
	}
	if plan.Ignored {
		logr.FromContextOrDiscard(ctx).V(1).Info("capture observation ignored",
			"trigger", o.Trigger, "session", o.Session, "turn", o.Turn, "tool_use_id", o.ExternalCall)
		return nil
	}
	logr.FromContextOrDiscard(ctx).V(1).Info("capture plan prepared", "elapsed", time.Since(stage))
	stage = time.Now()
	states := []domain.State{}
	for _, sn := range plan.Snapshots {
		states = append(states, sn.State)
	}
	prepared, e := s.prepareIO(ctx, o.Worktree.ID, states)
	if e != nil {
		return e
	}
	logr.FromContextOrDiscard(ctx).V(1).Info("capture objects prepared", "elapsed", time.Since(stage), "new_objects", len(prepared.local))
	stage = time.Now()
	e = s.update(ctx, func(tx *gorm.DB) error {
		if e := s.publishObjects(tx, o.Worktree.ID, prepared); e != nil {
			return e
		}
		return s.commitCapture(tx, plan, prepared)
	})
	logr.FromContextOrDiscard(ctx).V(1).Info("capture transaction finished", "elapsed", time.Since(stage))
	return e
}
func (s *Store) commitCapture(tx *gorm.DB, p domain.CapturePlan, prepared *PreparedStates) error {
	if p.Ignored {
		return nil
	}
	o := p.Observation
	w := p.Worktree.ID
	if o.Receipt != "" {
		var r receiptRecord
		e := tx.Where("worktree_id = ? AND key = ?", w, o.Receipt).Take(&r).Error
		if e == nil {
			if r.Digest != o.Digest {
				return fmt.Errorf("hook receipt conflict")
			}
			return nil
		}
		if !errors.Is(e, ErrNotFound) {
			return e
		}
	}
	c, e := cursor(tx, w)
	if e != nil && !errors.Is(e, ErrNotFound) {
		return e
	}
	if c.Version != p.ExpectedVersion {
		return domain.ErrCaptureConflict
	}
	if o.Session != "" && o.Turn != "" {
		id := ContentHash([]byte(w + "/" + o.Session + "/" + o.Turn))
		if p.Call != nil {
			id = p.Call.TurnID
		}
		r := turnRecord{ID: id, WorktreeID: w, Session: o.Session, ExternalID: o.Turn}
		if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&r).Error; e != nil {
			return e
		}
	}
	if p.Call != nil {
		r := callRow(*p.Call)
		if e = tx.Create(&r).Error; e != nil {
			return e
		}
	}
	for _, t := range p.Calls {
		if e = tx.Model(&callRecord{}).Where("id = ?", t.ID).Update("status", t.Status).Error; e != nil {
			return e
		}
	}
	for _, sn := range p.Snapshots {
		if !prepared.roots[sn.State.Root()] {
			return fmt.Errorf("snapshot root was not prepared")
		}
		r := snapshotRecord{ID: sn.ID, WorktreeID: w, Base: sn.Base, Trigger: sn.Trigger, ObservedAt: sn.Time, Root: sn.State.Root()}
		if e = tx.Create(&r).Error; e != nil {
			return e
		}
	}
	var latest eventRecord
	e = tx.Where("worktree_id = ? AND projected = ?", w, false).Order("sequence DESC").Take(&latest).Error
	if e != nil && !errors.Is(e, ErrNotFound) {
		return e
	}
	for i, change := range p.Changes {
		r := eventRecord{ID: change.ID, WorktreeID: w, Sequence: latest.Sequence + int64(i) + 1, Kind: change.Kind, Segment: change.Segment, Base: change.Base, TurnID: change.TurnID, CallID: change.CallID, BeforeID: change.BeforeID, AfterID: change.AfterID, PairID: change.PairID, IntervalStart: change.From, IntervalEnd: change.Until}
		if e = tx.Create(&r).Error; e != nil {
			return e
		}
	}
	for _, a := range p.Associations {
		if e = associate(tx, []string{a.ChangeID}, a.Commit); e != nil {
			return e
		}
	}
	c = p.Cursor
	r := cursorRecord{WorktreeID: w, SnapshotID: c.SnapshotID, Base: c.Base, ActiveCall: c.ActiveCall, Segment: c.Segment, Version: c.Version}
	if e = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "worktree_id"}}, DoUpdates: clause.AssignmentColumns([]string{"snapshot_id", "base", "active_call", "segment", "version"})}).Create(&r).Error; e != nil {
		return e
	}
	if len(p.Changes) > 0 || len(p.Associations) > 0 {
		if e = bumpSource(tx, w); e != nil {
			return e
		}
	}
	segments := map[string]bool{}
	for _, change := range p.Changes {
		segments[change.Segment] = true
	}
	for segment := range segments {
		if e = autoUnknown(tx, w, segment); e != nil {
			return e
		}
	}
	if o.Receipt != "" {
		return tx.Create(&receiptRecord{WorktreeID: w, Key: o.Receipt, Digest: o.Digest}).Error
	}
	return nil
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

// HasReceipt is a fast preflight; commitCapture still checks atomically.
func (s *Store) HasReceipt(ctx context.Context, w, key, digest string) (bool, error) {
	if key == "" {
		return false, nil
	}
	var r receiptRecord
	e := s.db.WithContext(ctx).Where("worktree_id = ? AND key = ?", w, key).Take(&r).Error
	if errors.Is(e, ErrNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if r.Digest != digest {
		return false, fmt.Errorf("hook receipt conflict")
	}
	return true, nil
}
