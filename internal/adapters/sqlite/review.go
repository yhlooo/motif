package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yhlooo/motif/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStaleView = fmt.Errorf("review view changed; reload before continuing")

func (s *Store) SaveReview(ctx context.Context, w domain.Worktree, r *domain.Review) error {
	return s.PublishReview(ctx, w, r, 0)
}

// projectionPayload stores only immutable metadata and references, never whole states.
type projectionPayload struct {
	Review        domain.Review
	Before, After []string
}

func (s *Store) PublishReview(ctx context.Context, w domain.Worktree, r *domain.Review, expected int64) error {
	release, leaseErr := s.lease(false)
	if leaseErr != nil {
		return leaseErr
	}
	defer release()

	for i := 1; i < len(r.Items); i++ {
		if !domain.Equal(r.Items[i-1].After, r.Items[i].Before) {
			return fmt.Errorf("projection state chain is discontinuous")
		}
	}
	states := []domain.State{}
	for _, item := range r.Items {
		states = append(states, item.Before, item.After)
	}
	prepared, err := s.prepareIO(ctx, w.ID, states)
	if err != nil {
		return err
	}
	return s.update(ctx, func(tx *gorm.DB) error {
		if e := s.publishObjects(tx, w.ID, prepared); e != nil {
			return e
		}

		if r.CheckSource {
			var v repositoryRecord
			if e := tx.Where("id = ?", w.RepositoryID).Take(&v).Error; e != nil {
				return e
			}
			if v.Generation != r.SourceRevision {
				return ErrStaleView
			}
		}
		var old reviewRecord
		e := tx.Where("id = ? AND repository_id = ?", r.ID, w.RepositoryID).Take(&old).Error
		if e != nil && e != gorm.ErrRecordNotFound {
			return e
		}
		if e == nil && old.Version != expected {
			return ErrStaleView
		}
		if e == gorm.ErrRecordNotFound && expected != 0 {
			return ErrNotFound
		}
		if expected > 0 && old.ViewDigest == r.ViewDigest {
			r.Version = old.Version
			r.ProjectionID = old.ProjectionID
			return tx.Model(&old).Updates(map[string]any{"input_signature": r.InputSignature, "source_revision": r.SourceRevision}).Error
		}
		r.Version = expected + 1
		r.ProjectionID = domain.ID()
		diagnostics, e := json.Marshal(r.Diagnostics)
		if e != nil {
			return e
		}
		row := reviewRecord{ProjectionID: r.ProjectionID, InputSignature: r.InputSignature, ComparisonMode: r.ComparisonMode, History: r.History, Diagnostics: string(diagnostics), SourceRevision: r.SourceRevision, ID: r.ID, RepositoryID: w.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning, Version: r.Version, ViewDigest: r.ViewDigest}
		if expected == 0 {
			if e = tx.Create(&row).Error; e != nil {
				return e
			}
		} else {
			if e = tx.Model(&old).Select("projection_id", "input_signature", "source_revision", "source_oid", "target_oid", "base", "head", "integrity", "warning", "version", "view_digest", "history", "diagnostics", "comparison_mode").Updates(&row).Error; e != nil {
				return e
			}
		}
		payload := projectionPayload{Before: []string{}, After: []string{}}
		for i := range r.Items {
			item := &r.Items[i]
			key := item.StableKey
			if key == "" {
				return fmt.Errorf("review item requires stable identity")
			}
			item.ID = ContentHash([]byte(r.ID + "/" + key))
			identity := reviewItemRecord{ID: item.ID, ReviewID: r.ID, StableKey: key}
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&identity).Error; e != nil {
				return e
			}
			decision := decisionRecord{ItemID: item.ID, Decision: "pending", Revision: 1}
			if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&decision).Error; e != nil {
				return e
			}
			b, e := s.saveSnapshot(tx, w.ID, r.Base, "projection", item.Before, prepared)
			if e != nil {
				return e
			}
			a, e := s.saveSnapshot(tx, w.ID, r.Head, "projection", item.After, prepared)
			if e != nil {
				return e
			}
			item.BeforeID, item.AfterID = b.ID, a.ID
			payload.Before = append(payload.Before, b.ID)
			payload.After = append(payload.After, a.ID)
		}
		payload.Review = *r
		raw, e := json.Marshal(payload)
		if e != nil {
			return e
		}
		return tx.Create(&reviewProjectionRecord{ID: r.ProjectionID, ReviewID: r.ID, Version: r.Version, Payload: raw}).Error
	})
}

func (s *Store) Reviews(rid string) ([]domain.Review, error) {
	var ids []string
	if err := s.db.Model(&reviewRecord{}).Where("repository_id = ?", rid).Order("created DESC").Order("id").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Review, 0, len(ids))
	for _, id := range ids {
		r, err := s.ReviewSummary(rid, id)
		if err != nil {
			return nil, err
		}
		for i := range r.Items {
			r.Items[i].Events = nil
		}
		out = append(out, r)
	}
	return out, nil
}
func (s *Store) ReviewSummary(rid, id string) (result domain.Review, err error) {
	err = s.update(context.Background(), func(tx *gorm.DB) error { var e error; result, e = s.readReview(tx, rid, id); return e })
	return
}
func (s *Store) Review(rid, id string) (result domain.Review, err error) {
	result, err = s.ReviewSummary(rid, id)
	if err != nil {
		return
	}
	return s.LoadReviewStates(result)
}

// LoadReviewStates resolves immutable snapshot references after the DB read ends.
func (s *Store) LoadReviewStates(result domain.Review) (domain.Review, error) {
	for i := range result.Items {
		item := &result.Items[i]
		b, e := s.Snapshot(item.BeforeID)
		if e != nil {
			return result, e
		}
		a, e := s.Snapshot(item.AfterID)
		if e != nil {
			return result, e
		}
		item.Before, item.After = b.State, a.State
	}
	return result, nil
}
func (s *Store) readReview(q *gorm.DB, rid, id string) (domain.Review, error) {
	var row reviewRecord
	if e := q.Where("id = ? AND repository_id = ?", id, rid).Take(&row).Error; e != nil {
		return domain.Review{}, e
	}
	return s.readProjection(q, row, row.ProjectionID)
}
func (s *Store) readProjection(q *gorm.DB, row reviewRecord, projectionID string) (domain.Review, error) {
	var record reviewProjectionRecord
	if e := q.Where("id = ? AND review_id = ?", projectionID, row.ID).Take(&record).Error; e != nil {
		return domain.Review{}, e
	}
	var payload projectionPayload
	if e := json.Unmarshal(record.Payload, &payload); e != nil {
		return domain.Review{}, e
	}
	r := payload.Review
	if len(payload.Before) != len(r.Items) || len(payload.After) != len(r.Items) {
		return domain.Review{}, fmt.Errorf("corrupt projection references")
	}
	r.InputSignature = row.InputSignature
	r.SourceRevision = row.SourceRevision
	r.Refresh = row.value().Refresh
	ids := []string{}
	for _, item := range r.Items {
		ids = append(ids, item.ID)
	}
	var decisions []decisionRecord
	if len(ids) > 0 {
		if e := q.Where("item_id IN ?", ids).Find(&decisions).Error; e != nil {
			return r, e
		}
	}
	byID := map[string]decisionRecord{}
	for _, d := range decisions {
		byID[d.ItemID] = d
	}
	r.Status = "approved"
	if len(r.Items) == 0 {
		r.Status = "in_review"
	}
	pending, rejected := false, false
	for i := range r.Items {
		item := &r.Items[i]
		d, ok := byID[item.ID]
		if !ok {
			return r, fmt.Errorf("missing decision identity")
		}
		item.Decision = d.Decision
		item.DecisionRevision = d.Revision
		item.BeforeID = payload.Before[i]
		item.AfterID = payload.After[i]
		pending = pending || d.Decision == "pending"
		rejected = rejected || d.Decision == "rejected"
	}
	if rejected {
		r.Status = "changes_requested"
	} else if pending {
		r.Status = "in_review"
	}
	return r, nil
}
func (s *Store) PublishedReview(rid, id, projectionID string) (result domain.Review, err error) {
	err = s.update(context.Background(), func(tx *gorm.DB) error {
		var row reviewRecord
		if e := tx.Where("id = ? AND repository_id = ?", id, rid).Take(&row).Error; e != nil {
			return e
		}
		var e error
		result, e = s.readProjection(tx, row, projectionID)
		return e
	})
	return
}
func (s *Store) Decision(ctx context.Context, rid, review, item, decision string, versions ...int64) error {
	if len(versions) != 1 {
		return ErrStaleView
	}
	return s.Decisions(ctx, rid, review, []string{item}, decision, versions[0])
}
func (s *Store) Threads(rid, review string) ([]domain.Thread, error) {
	var exists reviewRecord
	if err := s.db.Select("id").Where("id = ? AND repository_id = ?", review, rid).Take(&exists).Error; err != nil {
		return nil, err
	}
	var rows []threadRecord
	if err := s.db.Where("review_id = ?", review).Order("rowid").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.Thread, 0, len(rows))
	for _, row := range rows {
		thread := domain.Thread{AnchorItem: row.AnchorItem, BeforeID: row.BeforeID, AfterID: row.AfterID, ID: row.ID, ReviewID: row.ReviewID, First: row.FirstItem, Last: row.LastItem, Path: string(row.Path), Side: row.Side, Start: row.StartLine, End: row.EndLine, ContentRef: row.ContentRef, Comments: []domain.Comment{}}
		var comments []commentRecord
		if err := s.db.Where("thread_id = ?", row.ID).Order("created").Order("rowid").Find(&comments).Error; err != nil {
			return nil, err
		}
		for _, c := range comments {
			thread.Comments = append(thread.Comments, domain.Comment{ID: c.ID, Body: c.Body, Author: c.Author, Created: c.Created})
		}
		out = append(out, thread)
	}
	return out, nil
}
func (s *Store) AddThread(ctx context.Context, rid string, t domain.Thread, body, author string, versions ...int64) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		var row reviewRecord
		if err := tx.Where("id = ? AND repository_id = ?", t.ReviewID, rid).Take(&row).Error; err != nil {
			return err
		}
		if len(versions) != 1 || row.Version != versions[0] {
			return ErrStaleView
		}
		cr, e := s.readReview(tx, rid, t.ReviewID)
		if e != nil {
			return e
		}
		first, last := domain.ReviewItem{}, domain.ReviewItem{}
		firstPosition, lastPosition := -1, -1
		for i, item := range cr.Items {
			if item.ID == t.First {
				first = item
				firstPosition = i
			}
			if item.ID == t.Last {
				last = item
				lastPosition = i
			}
		}
		if firstPosition < 0 || lastPosition < firstPosition {
			return ErrStaleView
		}
		t.BeforeID = first.BeforeID
		t.AfterID = last.AfterID
		if t.AnchorItem == "" {
			if t.Side == "old" {
				t.AnchorItem = t.First
			} else {
				t.AnchorItem = t.Last
			}
		}
		thread := threadRecord{AnchorItem: t.AnchorItem, BeforeID: t.BeforeID, AfterID: t.AfterID, ID: t.ID, ReviewID: t.ReviewID, FirstItem: t.First, LastItem: t.Last, Path: []byte(t.Path), Side: t.Side, StartLine: t.Start, EndLine: t.End, ContentRef: t.ContentRef}
		if err := tx.Create(&thread).Error; err != nil {
			return err
		}
		return tx.Create(&commentRecord{ID: domain.ID(), ThreadID: t.ID, Body: body, Author: author, Created: domain.Now()}).Error
	})
}
func (s *Store) Reply(ctx context.Context, rid, thread, body, author string, versions ...int64) error {
	if len(versions) != 1 {
		return ErrStaleView
	}
	return s.update(ctx, func(tx *gorm.DB) error {
		scope := tx.Model(&reviewRecord{}).Select("id").Where("repository_id = ?", rid)
		var row threadRecord
		if err := tx.Select("id", "review_id", "anchor_item", "first_item", "last_item", "side").Where("id = ? AND review_id IN (?)", thread, scope).Take(&row).Error; err != nil {
			return err
		}
		if len(versions) > 0 {
			var cr reviewRecord
			if err := tx.Where("id = ?", row.ReviewID).Take(&cr).Error; err != nil {
				return err
			}
			if versions[0] <= 0 || versions[0] != cr.Version {
				return ErrStaleView
			}
			if row.AnchorItem == "" {
				row.AnchorItem = row.LastItem
				if row.Side == "old" {
					row.AnchorItem = row.FirstItem
				}
			}
			current, e := s.readReview(tx, rid, cr.ID)
			if e != nil {
				return e
			}
			active := false
			for _, item := range current.Items {
				active = active || item.ID == row.AnchorItem
			}
			if !active {
				return ErrStaleView
			}
		}
		return tx.Create(&commentRecord{ID: domain.ID(), ThreadID: thread, Body: body, Author: author, Created: domain.Now()}).Error
	})
}

// ReviewInputs reads one committed source revision, including cross-worktree
// batch membership and rewrite projections. Publish refuses a stale revision.
func (s *Store) ReviewInputs(ctx context.Context, rid string, commits []string) (result []domain.Event, projected map[string]bool, warnings []string, revision int64, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		var v repositoryRecord
		if e := tx.Where("id = ?", rid).Take(&v).Error; e != nil {
			return e
		}
		revision = v.Generation
		var worktrees []string
		if e := tx.Model(&worktreeRecord{}).Where("repository_id = ?", rid).Order("root").Pluck("id", &worktrees).Error; e != nil {
			return e
		}
		result = []domain.Event{}
		for _, w := range worktrees {
			list, e := events(tx, w)
			if e != nil {
				return e
			}
			result = append(result, list...)
		}
		var rows []projectionRecord
		if e := tx.Where("repository_id = ?", rid).Find(&rows).Error; e != nil {
			return e
		}
		selected := map[string]bool{}
		for _, id := range commits {
			selected[id] = true
		}
		projected = map[string]bool{}
		warnings = []string{}
		for _, row := range rows {
			projected[row.Commit] = true
			if selected[row.Commit] && row.Warning != "" {
				warnings = append(warnings, row.Warning)
			}
		}
		return nil
	})
	return
}

func (s *Store) SourceRevision(ctx context.Context, rid string) (int64, error) {
	var v repositoryRecord
	e := s.db.WithContext(ctx).Where("id = ?", rid).Take(&v).Error
	return v.Generation, e
}
func (s *Store) BeginRefresh(ctx context.Context, rid, id string) (attempt int64, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		var row reviewRecord
		if e := tx.Where("repository_id = ? AND id = ?", rid, id).Take(&row).Error; e != nil {
			return e
		}
		attempt = row.RefreshAttempt + 1
		return tx.Model(&row).Updates(map[string]any{"refresh_attempt": attempt, "refresh_attempted_at": domain.Now()}).Error
	})
	return
}
func (s *Store) FinishRefresh(ctx context.Context, rid, id string, attempt int64, code, message string) error {
	fields := map[string]any{"refresh_code": code, "refresh_message": message}
	if code == "ok" {
		fields["refresh_success_at"] = domain.Now()
	}
	return s.db.WithContext(ctx).Model(&reviewRecord{}).Where("repository_id = ? AND id = ? AND refresh_attempt = ?", rid, id, attempt).Updates(fields).Error
}

// Decisions atomically changes an entire selection at one published version.
func (s *Store) Decisions(ctx context.Context, rid, review string, items []string, decision string, version int64, expected ...map[string]int64) error {
	if len(items) == 0 || (decision != "pending" && decision != "approved" && decision != "rejected") {
		return ErrInvalidArgument
	}
	return s.update(ctx, func(tx *gorm.DB) error {
		var cr reviewRecord
		if err := tx.Where("id = ? AND repository_id = ?", review, rid).Take(&cr).Error; err != nil {
			return err
		}
		if version <= 0 || version != cr.Version {
			return ErrStaleView
		}
		current, e := s.readReview(tx, rid, review)
		if e != nil {
			return e
		}
		active := map[string]bool{}
		for _, item := range current.Items {
			active[item.ID] = true
		}
		if len(expected) > 0 {
			revisions := map[string]int64{}
			for _, item := range current.Items {
				revisions[item.ID] = item.DecisionRevision
			}
			for _, id := range items {
				revision, ok := expected[0][id]
				if !ok || revision <= 0 || revision != revisions[id] {
					return ErrStaleView
				}
			}
		}
		unique := map[string]bool{}
		for _, id := range items {
			if unique[id] {
				return ErrInvalidArgument
			}
			unique[id] = true
			if !active[id] {
				return ErrNotFound
			}
		}
		return tx.Model(&decisionRecord{}).Where("item_id IN ?", items).Updates(map[string]any{"decision": decision, "revision": gorm.Expr("revision + 1")}).Error
	})
}

func (s *Store) LoadSelection(r domain.Review, first, last string, middle bool) (domain.Review, error) {
	a, b := -1, -1
	for i, item := range r.Items {
		if item.ID == first {
			a = i
		}
		if item.ID == last {
			b = i
		}
	}
	if a < 0 || b < a {
		return r, ErrInvalidArgument
	}
	for i := a; i <= b; i++ {
		item := &r.Items[i]
		if middle || i == a {
			sn, e := s.Snapshot(item.BeforeID)
			if e != nil {
				return r, e
			}
			item.Before = sn.State
		}
		if middle || i == b {
			sn, e := s.Snapshot(item.AfterID)
			if e != nil {
				return r, e
			}
			item.After = sn.State
		}
	}
	return r, nil
}
