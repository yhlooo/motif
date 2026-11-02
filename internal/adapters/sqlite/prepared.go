package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
	"github.com/yhlooo/motif/internal/domain/state"
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
	var ownershipErr error
	owned := func(id string) bool {
		if p.local[id] || known[id] {
			return true
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
		if p.roots[snapshotState.Root()] {
			continue
		}
		e := snapshotState.Persist(owned, func(id string, data []byte) error {
			if ownershipErr != nil {
				return ownershipErr
			}
			var node state.NodeRecord
			if e := json.Unmarshal(data, &node); e != nil {
				return e
			}
			ref := node.File.ContentRef
			if ref != "" && !content.IsGit(ref) && !owned(ref) {
				if ownershipErr != nil {
					return ownershipErr
				}
				if _, e := s.Blob(ref); e != nil {
					return e
				}
				p.local[ref] = true
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
