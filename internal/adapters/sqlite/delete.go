package sqlite

import (
	"context"
	"fmt"
	"os"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeletionPlan struct {
	Repository     string         `json:"repository"`
	Worktrees      []string       `json:"worktrees"`
	Counts         map[string]int `json:"counts"`
	ExclusiveBlobs int            `json:"exclusive_blobs"`
	SharedBlobs    int            `json:"shared_blobs"`
}

func worktreeIDs(q *gorm.DB, rid string) *gorm.DB {
	return q.Model(&worktreeRecord{}).Select("id").Where("repository_id = ?", rid)
}
func snapshotIDs(q *gorm.DB, rid string) *gorm.DB {
	return q.Model(&snapshotRecord{}).Select("id").Where("worktree_id IN (?)", worktreeIDs(q, rid))
}
func reviewIDs(q *gorm.DB, rid string) *gorm.DB {
	return q.Model(&reviewRecord{}).Select("id").Where("repository_id = ?", rid)
}
func (s *Store) DeletionPlan(rid string) (DeletionPlan, error) {
	p := DeletionPlan{Repository: rid, Worktrees: []string{}, Counts: map[string]int{}}
	if err := s.db.Model(&worktreeRecord{}).Where("repository_id = ?", rid).Pluck("root", &p.Worktrees).Error; err != nil {
		return p, err
	}
	wids := worktreeIDs(s.db, rid)
	reviews := reviewIDs(s.db, rid)
	turns := s.db.Model(&turnRecord{}).Select("id").Where("worktree_id IN (?)", wids)
	batches := s.db.Model(&batchRecord{}).Select("id").Where("worktree_id IN (?)", wids)
	threads := s.db.Model(&threadRecord{}).Select("id").Where("review_id IN (?)", reviews)
	queries := map[string]*gorm.DB{
		"commit_rewrites":       s.db.Model(&rewriteRecord{}).Where("repository_id = ?", rid),
		"commit_projections":    s.db.Model(&projectionRecord{}).Where("repository_id = ?", rid),
		"reference_transitions": s.db.Model(&referenceRecord{}).Where("repository_id = ?", rid),
		"worktrees":             s.db.Model(&worktreeRecord{}).Where("repository_id = ?", rid),
		"turns":                 s.db.Model(&turnRecord{}).Where("worktree_id IN (?)", wids),
		"snapshots":             s.db.Model(&snapshotRecord{}).Where("worktree_id IN (?)", wids),
		"object_refs":           s.db.Model(&objectReference{}).Where("repository_id = ?", rid),
		"events":                s.db.Model(&eventRecord{}).Where("worktree_id IN (?)", wids),
		"batches":               s.db.Model(&batchRecord{}).Where("worktree_id IN (?)", wids),
		"batch_members":         s.db.Model(&batchMemberRecord{}).Where("batch_id IN (?)", batches),
		"tool_calls":            s.db.Model(&callRecord{}).Where("turn_id IN (?)", turns),
		"hook_receipts":         s.db.Model(&receiptRecord{}).Where("worktree_id IN (?)", wids),
		"reviews":               s.db.Model(&reviewRecord{}).Where("repository_id = ?", rid),
		"comment_threads":       s.db.Model(&threadRecord{}).Where("review_id IN (?)", reviews),
		"comments":              s.db.Model(&commentRecord{}).Where("thread_id IN (?)", threads),
	}
	for name, q := range queries {
		var n int64
		if err := q.Count(&n).Error; err != nil {
			return p, err
		}
		p.Counts[name] = int(n)
	}
	hashes, err := repoHashes(s.db, rid)
	if err != nil {
		return p, err
	}

	for _, hash := range hashes {
		var n int64
		if err = s.db.Model(&objectReference{}).Where("hash = ? AND repository_id <> ?", hash, rid).Count(&n).Error; err != nil {
			return p, err
		}
		if n == 0 {
			p.ExclusiveBlobs++
		} else {
			p.SharedBlobs++
		}
	}
	return p, nil
}
func repoHashes(q *gorm.DB, rid string) ([]string, error) {
	var hashes []string
	e := q.Model(&objectReference{}).Where("repository_id = ?", rid).Pluck("hash", &hashes).Error
	return hashes, e
}
func (s *Store) DeleteRepository(ctx context.Context, rid string) error {
	release, e := s.lease(true)
	if e != nil {
		return e
	}
	defer release()
	e = s.update(ctx, func(tx *gorm.DB) error {
		hashes, e := repoHashes(tx, rid)
		if e != nil {
			return e
		}
		if e = tx.Where("id = ?", rid).Delete(&repositoryRecord{}).Error; e != nil {
			return e
		}
		for _, hash := range hashes {
			var n int64
			if e = tx.Model(&objectReference{}).Where("hash = ?", hash).Count(&n).Error; e != nil {
				return e
			}
			if n == 0 {
				if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&deletionTask{Hash: hash}).Error; e != nil {
					return e
				}
			}
		}
		return nil
	})
	if e != nil {
		return e
	}
	return s.drainDeletes()
}

// Caller holds the exclusive lifecycle lease. Cleanup is resumable after a crash.
func (s *Store) drainDeletes() error {
	var tasks []deletionTask
	if e := s.db.Find(&tasks).Error; e != nil {
		return e
	}
	for _, task := range tasks {
		var owners int64
		if e := s.db.Model(&objectReference{}).Where("hash = ?", task.Hash).Count(&owners).Error; e != nil {
			return e
		}
		if owners == 0 {
			path, e := s.blobPath(task.Hash)
			if e != nil {
				return e
			}
			if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
				return fmt.Errorf("project removed; content cleanup pending: %w", e)
			}
		}
		if e := s.db.Delete(&task).Error; e != nil {
			return e
		}
	}
	return nil
}
