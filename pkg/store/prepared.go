package store

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gorm.io/gorm"
)

// preparedIO contains immutable inputs resolved before opening a write transaction.
// A missing entry is an error: transactional code must never fall back to Git IO.
type preparedIO struct {
	bases     map[string]preparedBase
	snapshots map[string]Snapshot
	hashes    map[string]bool
	local     map[string]string
}
type preparedBase struct {
	state  State
	sparse bool
}
type preparedKey struct{}

func baseKey(w, oid string) string { return w + "/" + oid }
func (s *Store) prepareIO(ctx context.Context, w string, bases, ids []string, states []State) (context.Context, error) {
	p := &preparedIO{bases: map[string]preparedBase{}, snapshots: map[string]Snapshot{}, hashes: map[string]bool{}, local: map[string]string{}}
	for _, id := range ids {
		if id == "" {
			continue
		}
		sn, err := s.Snapshot(id)
		if err != nil {
			return ctx, err
		}
		p.snapshots[id] = sn
		states = append(states, sn.State)
		// The cursor may still be based on the preceding commit after post-commit.
		if sn.WorktreeID == w {
			bases = append(bases, sn.Base)
		}
	}
	for _, oid := range bases {
		key := baseKey(w, oid)
		if _, ok := p.bases[key]; ok {
			continue
		}
		state, sparse, err := s.baseState(s.db.WithContext(ctx), w, oid)
		if err != nil {
			return ctx, err
		}
		p.bases[key] = preparedBase{state, sparse}
	}
	for _, state := range states {
		for _, f := range state {
			if f.Hash != "" && !p.hashes[f.Hash] {
				if _, err := s.Blob(f.Hash); err != nil {
					return ctx, err
				}
				p.hashes[f.Hash] = true
				path, _ := s.blobPath(f.Hash)
				if _, err := os.Stat(path); err == nil {
					p.local[f.Hash] = path
				}
			}
		}
	}
	return context.WithValue(ctx, preparedKey{}, p), nil
}
func (s *Store) validatePrepared(tx *gorm.DB, state State) error {
	p, ok := tx.Statement.Context.Value(preparedKey{}).(*preparedIO)
	if !ok {
		return fmt.Errorf("snapshot IO was not prepared")
	}
	for _, f := range state {
		if path := p.local[f.Hash]; path != "" {
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("prepared content disappeared: %w", err)
			}
		}
		if f.Hash != "" && !p.hashes[f.Hash] {
			return fmt.Errorf("content not prepared: %s", f.Hash)
		}
	}
	return nil
}
func (s *Store) prepareObservation(ctx context.Context, o Observation, extraBases, extraIDs []string, states []State) (context.Context, error) {
	c, err := s.Cursor(o.Worktree.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return ctx, err
	}
	extraIDs = append(extraIDs, c.SnapshotID)
	extraBases = append(extraBases, o.Base, o.Commit)
	states = append(states, o.State, o.BaseState, o.CommitState)
	return s.prepareIO(ctx, o.Worktree.ID, extraBases, extraIDs, states)
}
