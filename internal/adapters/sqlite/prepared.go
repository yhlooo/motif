package sqlite

import (
	"context"
	"fmt"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PreparedStates contains durable roots and the newly acquired object references.
// The caller holds a shared lifecycle lease from preparation through publication.
type PreparedStates struct {
	roots map[string]bool
	local map[string]bool
}

func (s *Store) prepareIO(ctx context.Context, w string, states []domain.State) (*PreparedStates, error) {
	p := &PreparedStates{roots: map[string]bool{"": true}, local: map[string]bool{}}
	var worktree worktreeRecord
	if e := s.db.WithContext(ctx).Where("id = ?", w).Take(&worktree).Error; e != nil {
		return nil, e
	}
	known := map[string]bool{}
	// Preload sparse manifest/content ownership in bounded SQL batches.
	required := map[string]bool{}
	for _, st := range states {
		required[st.Root()] = true
		for _, entry := range st.Overrides() {
			if entry.File != nil {
				ref := entry.File.ContentRef
				if ref != "" && !content.IsGit(ref) {
					required[ref] = true
				}
			}
		}
	}
	ids := make([]string, 0, len(required))
	for id := range required {
		if id != "" {
			ids = append(ids, id)
			known[id] = false
		}
	}
	for start := 0; start < len(ids); start += 200 {
		end := min(start+200, len(ids))
		var ownedIDs []string
		if e := s.db.WithContext(ctx).Model(&objectReference{}).Where("repository_id = ? AND hash IN ?", worktree.RepositoryID, ids[start:end]).Pluck("hash", &ownedIDs).Error; e != nil {
			return nil, e
		}
		for _, id := range ownedIDs {
			known[id] = true
		}
	}
	var ownershipErr error
	owned := func(id string) bool {
		if p.local[id] {
			return true
		}
		if value, ok := known[id]; ok {
			return value
		}
		var n int64
		if e := s.db.WithContext(ctx).Model(&objectReference{}).Where("repository_id = ? AND hash = ?", worktree.RepositoryID, id).Count(&n).Error; e != nil {
			ownershipErr = e
			return false
		}
		known[id] = n > 0
		return n > 0
	}
	for _, snapshotState := range states {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if p.roots[snapshotState.Root()] {
			continue
		}
		// Only overlay content needs ownership; inherited Git entries are not copied.
		for _, entry := range snapshotState.Overrides() {
			if entry.File == nil {
				continue
			}
			ref := entry.File.ContentRef
			if ref != "" && !content.IsGit(ref) && !owned(ref) {
				if ownershipErr != nil {
					return nil, ownershipErr
				}
				if _, e := s.Blob(ref); e != nil {
					return nil, e
				}
				p.local[ref] = true
			}
		}
		e := snapshotState.Persist(owned, func(id string, data []byte) error {
			if ownershipErr != nil {
				return ownershipErr
			}
			ref, e := s.PutBlob(data)
			if e != nil {
				return e
			}
			if ref != id {
				return fmt.Errorf("state identity mismatch")
			}
			p.local[id] = true
			return nil
		})
		if e != nil {
			return nil, e
		}
		if ownershipErr != nil {
			return nil, ownershipErr
		}
		p.roots[snapshotState.Root()] = true
	}
	return p, nil
}
func (s *Store) validatePrepared(p *PreparedStates, state domain.State) error {
	if p == nil || !p.roots[state.Root()] {
		return fmt.Errorf("state was not prepared")
	}
	return nil
}
func (s *Store) publishObjects(tx *gorm.DB, w string, p *PreparedStates) error {
	var worktree worktreeRecord
	if e := tx.Where("id = ?", w).Take(&worktree).Error; e != nil {
		return e
	}
	refs := make([]objectReference, 0, len(p.local))
	for hash := range p.local {
		refs = append(refs, objectReference{RepositoryID: worktree.RepositoryID, Hash: hash})
	}
	if len(refs) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(refs, 100).Error
}
