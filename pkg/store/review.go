package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

func (s *Store) SaveReview(ctx context.Context, w Worktree, r *Review) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		row := reviewRecord{ID: r.ID, RepositoryID: w.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for i, oid := range r.Commits {
			if err := tx.Create(&reviewCommitRecord{ReviewID: r.ID, Position: i, OID: oid}).Error; err != nil {
				return err
			}
		}
		for i, item := range r.Items {
			if err := s.validateState(item.Before); err != nil {
				return err
			}
			if err := s.validateState(item.After); err != nil {
				return err
			}
			before, err := saveSnapshot(tx, w.ID, r.Base, "review", item.Before)
			if err != nil {
				return err
			}
			after, err := saveSnapshot(tx, w.ID, r.Head, "review", item.After)
			if err != nil {
				return err
			}
			record := reviewItemRecord{ID: item.ID, ReviewID: r.ID, Position: i, Kind: item.Kind, Title: item.Title, Description: item.Description, Decision: "pending", BeforeID: before.ID, AfterID: after.ID}
			if err = tx.Create(&record).Error; err != nil {
				return err
			}
			for j, id := range item.Events {
				if err = tx.Create(&reviewEventRecord{ItemID: item.ID, Position: j, EventID: id}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
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
func (s *Store) Review(rid, id string) (Review, error) {
	var row reviewRecord
	if err := s.db.Where("id = ? AND repository_id = ?", id, rid).Take(&row).Error; err != nil {
		return Review{}, err
	}
	r := row.value()
	r.Commits = []string{}
	r.Items = []ReviewItem{}
	if err := s.db.Model(&reviewCommitRecord{}).Where("review_id = ?", id).Order("position").Pluck("oid", &r.Commits).Error; err != nil {
		return r, err
	}
	var records []reviewItemRecord
	if err := s.db.Where("review_id = ?", id).Order("position").Find(&records).Error; err != nil {
		return r, err
	}
	r.Status = "approved"
	if len(records) == 0 {
		r.Status = "in_review"
	}
	pending, rejected := false, false
	for _, record := range records {
		before, err := s.Snapshot(record.BeforeID)
		if err != nil {
			return r, err
		}
		after, err := s.Snapshot(record.AfterID)
		if err != nil {
			return r, err
		}
		item := ReviewItem{ID: record.ID, Kind: record.Kind, Title: record.Title, Description: record.Description, Decision: record.Decision, Before: before.State, After: after.State, Events: []string{}}
		if err = s.db.Model(&reviewEventRecord{}).Where("item_id = ?", record.ID).Order("position").Pluck("event_id", &item.Events).Error; err != nil {
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
func (s *Store) Decision(ctx context.Context, rid, review, item, decision string) error {
	if decision != "pending" && decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid decision")
	}
	return s.update(ctx, func(tx *gorm.DB) error {
		scope := tx.Model(&reviewRecord{}).Select("id").Where("id = ? AND repository_id = ?", review, rid)
		result := tx.Model(&reviewItemRecord{}).Where("id = ? AND review_id IN (?)", item, scope).Update("decision", decision)
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
		thread := Thread{ID: row.ID, ReviewID: row.ReviewID, First: row.FirstItem, Last: row.LastItem, Path: string(row.Path), Side: row.Side, Start: row.StartLine, End: row.EndLine, Hash: row.Hash, Comments: []Comment{}}
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
func (s *Store) AddThread(ctx context.Context, rid string, t Thread, body, author string) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		var row reviewRecord
		if err := tx.Select("id").Where("id = ? AND repository_id = ?", t.ReviewID, rid).Take(&row).Error; err != nil {
			return err
		}
		thread := threadRecord{ID: t.ID, ReviewID: t.ReviewID, FirstItem: t.First, LastItem: t.Last, Path: []byte(t.Path), Side: t.Side, StartLine: t.Start, EndLine: t.End, Hash: t.Hash}
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
