package store

import (
	"context"
	"crypto/sha256"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RewriteMapping struct{ Old, New string }
type ProjectedChange struct {
	Origin        Event
	Before, After State
	Kind, PairID  string
}
type CommitProjection struct {
	Commit, Parent, Warning string
	Changes                 []ProjectedChange
}

func ProjectionBatchID(commit, origin string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(commit+"/"+origin)))
}
func (s *Store) ProjectedCommits(rid string) (map[string]bool, error) {
	var commits []string
	if err := s.db.Model(&projectionRecord{}).Where("repository_id = ?", rid).Pluck("commit_oid", &commits).Error; err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, oid := range commits {
		out[oid] = true
	}
	return out, nil
}
func (s *Store) SaveRewrite(ctx context.Context, w Worktree, kind string, mappings []RewriteMapping, projections []CommitProjection, pending []string, expected int64, resume Observation) error {
	bases := []string{}
	states := []State{}
	for _, p := range projections {
		bases = append(bases, p.Parent)
		for _, c := range p.Changes {
			states = append(states, c.Before, c.After)
		}
	}
	prepared, err := s.prepareObservation(ctx, resume, bases, nil, states)
	if err != nil {
		return err
	}
	return s.sourceUpdate(prepared, func(tx *gorm.DB) error {
		cursor, err := cursor(tx, w.ID)
		if err != nil {
			return err
		}
		if cursor.Version != expected {
			return fmt.Errorf("rewrite cursor changed; retry notification")
		}
		for i, m := range mappings {
			row := rewriteRecord{ID: ID(), RepositoryID: w.RepositoryID, Kind: kind, Old: m.Old, New: m.New, Position: i}
			if err = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "repository_id"}, {Name: "old"}, {Name: "new"}}, DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		var latest eventRecord
		if err = tx.Where("worktree_id = ?", w.ID).Order("sequence DESC").Take(&latest).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		seq := latest.Sequence
		for _, projection := range projections {
			var count int64
			if err = tx.Model(&projectionRecord{}).Where("repository_id = ? AND commit_oid = ?", w.RepositoryID, projection.Commit).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			projectionID := ID()
			record := projectionRecord{ID: projectionID, RepositoryID: w.RepositoryID, Commit: projection.Commit, Parent: projection.Parent, Warning: projection.Warning}
			if err = tx.Create(&record).Error; err != nil {
				return err
			}
			batchPositions := map[string]int{}
			lastUnknown := ""
			for _, change := range projection.Changes {
				if Equal(change.Before, change.After) {
					continue
				}
				if err = s.validatePrepared(tx, change.Before); err != nil {
					return err
				}
				if err = s.validatePrepared(tx, change.After); err != nil {
					return err
				}
				before, e := s.saveSnapshot(tx, w.ID, projection.Parent, "rewrite_projection", change.Before)
				if e != nil {
					return e
				}
				after, e := s.saveSnapshot(tx, w.ID, projection.Parent, "rewrite_projection", change.After)
				if e != nil {
					return e
				}
				seq++
				row := eventRecord{ID: ID(), WorktreeID: w.ID, Sequence: seq, Kind: change.Kind, Segment: projectionID, Base: projection.Parent, FinalCommit: projection.Commit, BeforeID: before.ID, AfterID: after.ID, PairID: change.PairID, OriginID: change.Origin.ID, Projected: true}
				if row.Kind == "tool_edit" {
					row.TurnID = change.Origin.TurnID
					row.CallID = change.Origin.CallID
				}
				if err = tx.Create(&row).Error; err != nil {
					return err
				}
				batchID, title, description, kind := "", "", "", ""
				if change.Origin.BatchID != "" && change.Kind == change.Origin.Kind {
					batchID = ProjectionBatchID(projection.Commit, change.Origin.BatchID)
					title = change.Origin.BatchTitle
					description = change.Origin.BatchDescription
					kind = "ai"
					if row.Kind == "unknown" {
						kind = "unknown"
					}
					lastUnknown = ""
				} else if row.Kind == "unknown" {
					if lastUnknown == "" {
						lastUnknown = ID()
					}
					batchID = lastUnknown
					title = "历史改写未知补齐"
					description = projection.Warning
					kind = "unknown"
				} else {
					lastUnknown = ""
				}
				if batchID != "" {
					position, exists := batchPositions[batchID]
					if !exists {
						if err = tx.Create(&batchRecord{ID: batchID, WorktreeID: w.ID, Kind: kind, Title: title, Description: description, OriginBatch: change.Origin.BatchID}).Error; err != nil {
							return err
						}
					}
					if err = tx.Create(&batchMemberRecord{BatchID: batchID, EventID: row.ID, Position: position}).Error; err != nil {
						return err
					}
					batchPositions[batchID] = position + 1
				}
			}
		}
		// Only the new association is filled; original baseline and snapshots remain.
		if len(pending) > 0 && len(mappings) == 1 {
			if err = tx.Model(&eventRecord{}).Where("id IN ? AND final_commit = ?", pending, "").Update("final_commit", mappings[0].New).Error; err != nil {
				return err
			}
		}
		return s.observeTx(tx, resume)
	})
}

type RefTransition struct{ Old, New, Ref string }

func (s *Store) RecordReferences(ctx context.Context, rid string, updates []RefTransition) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		for _, entry := range updates {
			if err := tx.Create(&referenceRecord{ID: ID(), RepositoryID: rid, Old: entry.Old, New: entry.New, Ref: []byte(entry.Ref), ObservedAt: Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ProjectionWarnings(rid string, commits []string) ([]string, error) {
	out := []string{}
	if len(commits) == 0 {
		return out, nil
	}
	err := s.db.Model(&projectionRecord{}).Where("repository_id = ? AND commit_oid IN ? AND warning <> ?", rid, commits, "").Pluck("warning", &out).Error
	return out, err
}
