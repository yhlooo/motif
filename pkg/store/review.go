package store

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

var ErrStaleView = fmt.Errorf("review view changed; reload before continuing")

func (s *Store) SaveReview(ctx context.Context, w Worktree, r *Review) error {
	return s.PublishReview(ctx, w, r, 0)
}

// PublishReview replaces only the active projection. Historical identities,
// decisions, original comment anchors and inactive items remain intact.
func (s *Store) PublishReview(ctx context.Context, w Worktree, r *Review, expected int64) error {
	ids := []string{}
	states := []State{}
	// Legacy matching uses immutable saved snapshots, loaded outside the transaction.
	var oldItems []reviewItemRecord
	if err := s.db.Where("review_id = ? AND stable_key = ?", r.ID, "").Find(&oldItems).Error; err != nil {
		return err
	}
	for _, item := range oldItems {
		ids = append(ids, item.BeforeID, item.AfterID)
	}
	for _, item := range r.Items {
		states = append(states, item.Before, item.After)
	}
	prepared, err := s.prepareIO(ctx, w.ID, []string{r.Base, r.Head}, ids, states)
	if err != nil {
		return err
	}
	return s.update(prepared, func(tx *gorm.DB) error {
		if r.CheckSource {
			var revision schemaMigration
			if err := tx.Take(&revision).Error; err != nil {
				return err
			}
			if revision.Generation != r.SourceRevision {
				return ErrStaleView
			}
		}
		var old reviewRecord
		err := tx.Where("id = ? AND repository_id = ?", r.ID, w.RepositoryID).Take(&old).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == nil && old.Version != expected {
			return ErrStaleView
		}
		if err == gorm.ErrRecordNotFound && expected != 0 {
			return ErrNotFound
		}
		if expected > 0 && old.ViewDigest == r.ViewDigest {
			r.Version = old.Version
			return tx.Model(&reviewRecord{}).Where("id = ?", r.ID).Updates(map[string]any{"source_revision": r.SourceRevision, "input_signature": r.InputSignature}).Error
		}
		r.Version = expected + 1
		diagnostics, _ := json.Marshal(r.Diagnostics)
		row := reviewRecord{InputSignature: r.InputSignature, ComparisonMode: r.ComparisonMode, History: r.History, Diagnostics: string(diagnostics), SourceRevision: r.SourceRevision, ID: r.ID, RepositoryID: w.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning, Version: r.Version, ViewDigest: r.ViewDigest}
		if expected == 0 {
			if err = tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Model(&reviewRecord{}).Where("id = ? AND version = ?", r.ID, expected).Select("input_signature", "comparison_mode", "history", "diagnostics", "source_oid", "target_oid", "base", "head", "integrity", "warning", "version", "view_digest", "source_revision").Updates(&row).Error; err != nil {
				return err
			}
		}
		if err = tx.Where("review_id = ?", r.ID).Delete(&reviewCommitRecord{}).Error; err != nil {
			return err
		}
		for i, oid := range r.Commits {
			if err = tx.Create(&reviewCommitRecord{ReviewID: r.ID, Position: i, OID: oid}).Error; err != nil {
				return err
			}
		}
		var existing []reviewItemRecord
		if err = tx.Where("review_id = ?", r.ID).Order("position").Find(&existing).Error; err != nil {
			return err
		}
		byKey := map[string]reviewItemRecord{}
		minimum := 0
		for _, item := range existing {
			if item.Position < minimum {
				minimum = item.Position
			}
			if item.StableKey != "" {
				byKey[item.StableKey] = item
			} else {
				// Legacy batch IDs were never persisted in the view. Matching
				// their text/members could approve a deleted-and-recreated batch.
				// Only aliases whose original identity is provable are adopted.
				for _, candidate := range r.Items {
					matched, e := s.sameLegacyItem(tx, old, item, candidate)
					if e != nil {
						return e
					}
					if matched {
						byKey[candidate.StableKey] = item
						break
					}
				}
			}
		}
		for _, item := range existing {
			if !item.Active {
				continue
			}
			minimum--
			if err = tx.Model(&reviewItemRecord{}).Where("id = ?", item.ID).Updates(map[string]any{"active": false, "position": minimum}).Error; err != nil {
				return err
			}
		}
		for i := range r.Items {
			item := &r.Items[i]
			if err = s.validatePrepared(tx, item.Before); err != nil {
				return err
			}
			if err = s.validatePrepared(tx, item.After); err != nil {
				return err
			}
			if previous, ok := byKey[item.StableKey]; ok {
				item.ID = previous.ID
				item.Decision = previous.Decision
			} else {
				item.Decision = "pending"
			}
			before, e := s.saveSnapshot(tx, w.ID, r.Base, "review", item.Before)
			if e != nil {
				return e
			}
			after, e := s.saveSnapshot(tx, w.ID, r.Head, "review", item.After)
			if e != nil {
				return e
			}
			record := reviewItemRecord{ID: item.ID, ReviewID: r.ID, Position: i, Kind: item.Kind, Title: item.Title, Description: item.Description, Decision: item.Decision, BeforeID: before.ID, AfterID: after.ID, StableKey: item.StableKey, Commit: item.Commit, SourceBatch: item.SourceBatch, Digest: item.Digest, Active: true}
			if _, ok := byKey[item.StableKey]; ok {
				if err = tx.Model(&reviewItemRecord{}).Where("id = ?", item.ID).Select("position", "kind", "title", "description", "before_id", "after_id", "digest", "active", "stable_key", "commit", "source_batch").Updates(&record).Error; err != nil {
					return err
				}
			} else {
				if err = tx.Create(&record).Error; err != nil {
					return err
				}
			}
			if err = tx.Where("item_id = ?", item.ID).Delete(&reviewEventRecord{}).Error; err != nil {
				return err
			}
			for j, eid := range item.Events {
				if err = tx.Create(&reviewEventRecord{ItemID: item.ID, Position: j, EventID: eid}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// sameLegacyItem never guesses a missing source batch identity.
func (s *Store) sameLegacyItem(tx *gorm.DB, review reviewRecord, previous reviewItemRecord, next ReviewItem) (bool, error) {
	if previous.Kind == "batch" || previous.Kind != next.Kind || next.SourceBatch != "" {
		return false, nil
	}
	var events []string
	if err := tx.Model(&reviewEventRecord{}).Where("item_id = ?", previous.ID).Order("position").Pluck("event_id", &events).Error; err != nil {
		return false, err
	}
	if !slices.Equal(events, next.Events) {
		return false, nil
	}
	if len(events) == 0 {
		if previous.Kind != "integrity_unknown" || review.Head != next.Commit {
			return false, nil
		}
	} else {
		var count int64
		if err := tx.Model(&eventRecord{}).Where("id IN ? AND final_commit = ?", events, next.Commit).Count(&count).Error; err != nil {
			return false, err
		}
		if count != int64(len(events)) {
			return false, nil
		}
	}
	before, err := s.snapshot(tx, previous.BeforeID)
	if err != nil {
		return false, err
	}
	after, err := s.snapshot(tx, previous.AfterID)
	if err != nil {
		return false, err
	}
	return Equal(before.State, next.Before) && Equal(after.State, next.After), nil
}

func (s *Store) Reviews(rid string) ([]Review, error) {
	var ids []string
	if err := s.db.Model(&reviewRecord{}).Where("repository_id = ?", rid).Order("created DESC").Order("id").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := make([]Review, 0, len(ids))
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
func (s *Store) ReviewSummary(rid, id string) (result Review, err error) {
	err = s.update(context.Background(), func(tx *gorm.DB) error { var e error; result, e = s.readReview(tx, rid, id); return e })
	return
}
func (s *Store) Review(rid, id string) (result Review, err error) {
	result, err = s.ReviewSummary(rid, id)
	if err != nil {
		return
	}
	return s.LoadReviewStates(result)
}

// LoadReviewStates resolves immutable snapshot references after the DB read ends.
func (s *Store) LoadReviewStates(result Review) (Review, error) {
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
func (s *Store) readReview(q *gorm.DB, rid, id string) (Review, error) {
	var row reviewRecord
	if err := q.Where("id = ? AND repository_id = ?", id, rid).Take(&row).Error; err != nil {
		return Review{}, err
	}
	r := row.value()
	r.Commits = []string{}
	r.Items = []ReviewItem{}
	if err := q.Model(&reviewCommitRecord{}).Where("review_id = ?", id).Order("position").Pluck("oid", &r.Commits).Error; err != nil {
		return r, err
	}
	var records []reviewItemRecord
	if err := q.Where("review_id = ? AND active = ?", id, true).Order("position").Find(&records).Error; err != nil {
		return r, err
	}
	r.Status = "approved"
	if len(records) == 0 {
		r.Status = "in_review"
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	members := []reviewEventRecord{}
	if len(ids) > 0 {
		if err := q.Where("item_id IN ?", ids).Order("item_id, position").Find(&members).Error; err != nil {
			return r, err
		}
	}
	eventsByItem := map[string][]string{}
	for _, member := range members {
		eventsByItem[member.ItemID] = append(eventsByItem[member.ItemID], member.EventID)
	}
	pending, rejected := false, false
	for _, record := range records {
		item := ReviewItem{BeforeID: record.BeforeID, AfterID: record.AfterID, StableKey: record.StableKey, Commit: record.Commit, SourceBatch: record.SourceBatch, Digest: record.Digest, ID: record.ID, Kind: record.Kind, Title: record.Title, Description: record.Description, Decision: record.Decision, Events: []string{}}
		if events := eventsByItem[record.ID]; events != nil {
			item.Events = events
		}
		pending = pending || item.Decision == "pending"
		rejected = rejected || item.Decision == "rejected"
		r.Items = append(r.Items, item)
	}
	if rejected {
		r.Status = "changes_requested"
	} else if pending {
		r.Status = "in_review"
	}
	return r, nil
}
func (s *Store) Decision(ctx context.Context, rid, review, item, decision string, versions ...int64) error {
	if decision != "pending" && decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid decision")
	}
	return s.update(ctx, func(tx *gorm.DB) error {
		var cr reviewRecord
		if err := tx.Where("id = ? AND repository_id = ?", review, rid).Take(&cr).Error; err != nil {
			return err
		}
		if len(versions) > 0 && versions[0] != cr.Version {
			return ErrStaleView
		}
		result := tx.Model(&reviewItemRecord{}).Where("id = ? AND review_id = ? AND active = ?", item, review, true).Update("decision", decision)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}
func (s *Store) Threads(rid, review string) ([]Thread, error) {
	var exists reviewRecord
	if err := s.db.Select("id").Where("id = ? AND repository_id = ?", review, rid).Take(&exists).Error; err != nil {
		return nil, err
	}
	var rows []threadRecord
	if err := s.db.Where("review_id = ?", review).Order("rowid").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Thread, 0, len(rows))
	for _, row := range rows {
		thread := Thread{AnchorItem: row.AnchorItem, BeforeID: row.BeforeID, AfterID: row.AfterID, ID: row.ID, ReviewID: row.ReviewID, First: row.FirstItem, Last: row.LastItem, Path: string(row.Path), Side: row.Side, Start: row.StartLine, End: row.EndLine, Hash: row.Hash, Comments: []Comment{}}
		var comments []commentRecord
		if err := s.db.Where("thread_id = ?", row.ID).Order("created").Order("rowid").Find(&comments).Error; err != nil {
			return nil, err
		}
		for _, c := range comments {
			thread.Comments = append(thread.Comments, Comment{ID: c.ID, Body: c.Body, Author: c.Author, Created: c.Created})
		}
		out = append(out, thread)
	}
	return out, nil
}
func (s *Store) AddThread(ctx context.Context, rid string, t Thread, body, author string, versions ...int64) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		var row reviewRecord
		if err := tx.Where("id = ? AND repository_id = ?", t.ReviewID, rid).Take(&row).Error; err != nil {
			return err
		}
		if len(versions) > 0 && row.Version != versions[0] {
			return ErrStaleView
		}
		first, last := reviewItemRecord{}, reviewItemRecord{}
		if err := tx.Where("id = ? AND review_id = ? AND active = ?", t.First, t.ReviewID, true).Take(&first).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND review_id = ? AND active = ?", t.Last, t.ReviewID, true).Take(&last).Error; err != nil {
			return err
		}
		if first.Position > last.Position {
			return fmt.Errorf("invalid comment selection")
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
		thread := threadRecord{AnchorItem: t.AnchorItem, BeforeID: t.BeforeID, AfterID: t.AfterID, ID: t.ID, ReviewID: t.ReviewID, FirstItem: t.First, LastItem: t.Last, Path: []byte(t.Path), Side: t.Side, StartLine: t.Start, EndLine: t.End, Hash: t.Hash}
		if err := tx.Create(&thread).Error; err != nil {
			return err
		}
		return tx.Create(&commentRecord{ID: ID(), ThreadID: t.ID, Body: body, Author: author, Created: Now()}).Error
	})
}
func (s *Store) Reply(ctx context.Context, rid, thread, body, author string, versions ...int64) error {
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
			var active int64
			if err := tx.Model(&reviewItemRecord{}).Where("id = ? AND review_id = ? AND active = ?", row.AnchorItem, cr.ID, true).Count(&active).Error; err != nil {
				return err
			}
			if active == 0 {
				return ErrStaleView
			}
		}
		return tx.Create(&commentRecord{ID: ID(), ThreadID: thread, Body: body, Author: author, Created: Now()}).Error
	})
}

// ReviewInputs reads one committed source revision, including cross-worktree
// batch membership and rewrite projections. Publish refuses a stale revision.
func (s *Store) ReviewInputs(ctx context.Context, rid string, commits []string) (result []Event, projected map[string]bool, warnings []string, revision int64, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		var v schemaMigration
		if e := tx.Take(&v).Error; e != nil {
			return e
		}
		revision = v.Generation
		var worktrees []string
		if e := tx.Model(&worktreeRecord{}).Where("repository_id = ?", rid).Order("root").Pluck("id", &worktrees).Error; e != nil {
			return e
		}
		result = []Event{}
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

func (s *Store) SourceRevision(ctx context.Context) (int64, error) {
	var v schemaMigration
	err := s.db.WithContext(ctx).Take(&v).Error
	return v.Generation, err
}
func (s *Store) BeginRefresh(ctx context.Context, rid, id string) (attempt int64, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		var row reviewRecord
		if e := tx.Where("repository_id = ? AND id = ?", rid, id).Take(&row).Error; e != nil {
			return e
		}
		attempt = row.RefreshAttempt + 1
		return tx.Model(&row).Updates(map[string]any{"refresh_attempt": attempt, "refresh_attempted_at": Now()}).Error
	})
	return
}
func (s *Store) FinishRefresh(ctx context.Context, rid, id string, attempt int64, code, message string) error {
	fields := map[string]any{"refresh_code": code, "refresh_message": message}
	if code == "ok" {
		fields["refresh_success_at"] = Now()
	}
	return s.db.WithContext(ctx).Model(&reviewRecord{}).Where("repository_id = ? AND id = ? AND refresh_attempt = ?", rid, id, attempt).Updates(fields).Error
}

// Decisions atomically changes an entire selection at one published version.
func (s *Store) Decisions(ctx context.Context, rid, review string, items []string, decision string, version int64) error {
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
		unique := map[string]bool{}
		for _, id := range items {
			unique[id] = true
		}
		var count int64
		query := tx.Model(&reviewItemRecord{}).Where("review_id = ? AND id IN ? AND active = ?", review, items, true)
		if err := query.Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(unique)) {
			return ErrNotFound
		}
		return tx.Model(&reviewItemRecord{}).Where("review_id = ? AND id IN ? AND active = ?", review, items, true).Update("decision", decision).Error
	})
}
