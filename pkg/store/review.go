package store

import (
	"context"
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
	return s.update(ctx, func(tx *gorm.DB) error {
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
			return nil
		}
		r.Version = expected + 1
		row := reviewRecord{SourceRevision: r.SourceRevision, ID: r.ID, RepositoryID: w.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning, Version: r.Version, ViewDigest: r.ViewDigest}
		if expected == 0 {
			if err = tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Model(&reviewRecord{}).Where("id = ? AND version = ?", r.ID, expected).Select("source_oid", "target_oid", "base", "head", "integrity", "warning", "version", "view_digest", "source_revision").Updates(&row).Error; err != nil {
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
			if err = s.validateState(item.Before); err != nil {
				return err
			}
			if err = s.validateState(item.After); err != nil {
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
		r, err := s.Review(rid, id)
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
func (s *Store) Review(rid, id string) (result Review, err error) {
	err = s.db.Transaction(func(tx *gorm.DB) error { var e error; result, e = s.readReview(tx, rid, id); return e })
	return
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
	pending, rejected := false, false
	for _, record := range records {
		before, err := s.snapshot(q, record.BeforeID)
		if err != nil {
			return r, err
		}
		after, err := s.snapshot(q, record.AfterID)
		if err != nil {
			return r, err
		}
		if err = s.validateState(before.State); err != nil {
			return r, err
		}
		if err = s.validateState(after.State); err != nil {
			return r, err
		}
		item := ReviewItem{StableKey: record.StableKey, Commit: record.Commit, SourceBatch: record.SourceBatch, Digest: record.Digest, ID: record.ID, Kind: record.Kind, Title: record.Title, Description: record.Description, Decision: record.Decision, Before: before.State, After: after.State, Events: []string{}}
		if err = q.Model(&reviewEventRecord{}).Where("item_id = ?", record.ID).Order("position").Pluck("event_id", &item.Events).Error; err != nil {
			return r, err
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
func (s *Store) Reply(ctx context.Context, rid, thread, body, author string) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		scope := tx.Model(&reviewRecord{}).Select("id").Where("repository_id = ?", rid)
		var row threadRecord
		if err := tx.Select("id").Where("id = ? AND review_id IN (?)", thread, scope).Take(&row).Error; err != nil {
			return err
		}
		return tx.Create(&commentRecord{ID: ID(), ThreadID: thread, Body: body, Author: author, Created: Now()}).Error
	})
}

// ReviewInputs reads one committed source revision, including cross-worktree
// batch membership and rewrite projections. Publish refuses a stale revision.
func (s *Store) ReviewInputs(ctx context.Context, rid string, commits []string) (result []Event, projected map[string]bool, warnings []string, revision int64, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
