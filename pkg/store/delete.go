package store

import (
	"context"
	"fmt"
	"os"

	"gorm.io/gorm"
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
		"snapshot_entries":      s.db.Model(&snapshotEntryRecord{}).Where("snapshot_id IN (?)", snapshotIDs(s.db, rid)),
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
	retainedWorktrees := s.db.Model(&worktreeRecord{}).Select("id").Where("repository_id <> ?", rid)
	retainedSnapshots := s.db.Model(&snapshotRecord{}).Select("id").Where("worktree_id IN (?)", retainedWorktrees)
	for _, hash := range hashes {
		var n int64
		if err = s.db.Model(&snapshotEntryRecord{}).Where("hash = ? AND snapshot_id IN (?)", hash, retainedSnapshots).Count(&n).Error; err != nil {
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
	out := []string{}
	err := q.Model(&snapshotEntryRecord{}).Distinct("hash").Where("snapshot_id IN (?) AND hash <> ?", snapshotIDs(q, rid), "").Pluck("hash", &out).Error
	return out, err
}
func (s *Store) DeleteRepository(ctx context.Context, rid string) error {
	// Stage exclusive blobs while holding the DB write lock. A later snapshot must
	// validate its blobs inside its own write transaction, so a prepared capture
	// can fail cleanly instead of committing a dangling reference during uninstall.
	staged := map[string]string{}
	err := s.update(ctx, func(tx *gorm.DB) error {
		hashes, err := repoHashes(tx, rid)
		if err != nil {
			return err
		}
		if err = tx.Where("id = ?", rid).Delete(&repositoryRecord{}).Error; err != nil {
			return err
		}
		for _, hash := range hashes {
			var count int64
			if err = tx.Model(&snapshotEntryRecord{}).Where("hash = ?", hash).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			path, err := s.blobPath(hash)
			if err != nil {
				return err
			}
			temporary := path + ".delete-" + ID()
			if err = os.Rename(path, temporary); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			staged[path] = temporary
		}
		return nil
	})
	if err != nil {
		for original, temporary := range staged {
			if e := os.Rename(temporary, original); e != nil {
				return fmt.Errorf("database rollback: %v; failed to restore blob: %w", err, e)
			}
		}
		return err
	}
	for _, temporary := range staged {
		if e := os.Remove(temporary); e != nil {
			return fmt.Errorf("project data deleted, but blob cleanup failed: %w", e)
		}
	}
	return nil
}
