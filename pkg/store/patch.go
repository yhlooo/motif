package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

var ErrInvalidArgument = errors.New("invalid argument")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, fmt.Sprintf(format, args...))
}

func events(q *gorm.DB, w string) ([]Event, error) {
	var rows []eventRecord
	if err := q.Where("worktree_id = ?", w).Order("sequence").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(rows))
	callIDs := []string{}
	for _, r := range rows {
		ids = append(ids, r.ID)
		if r.CallID != "" {
			callIDs = append(callIDs, r.CallID)
		}
	}
	var members []batchMemberRecord
	if err := q.Preload("Batch").Where("event_id IN ?", ids).Find(&members).Error; err != nil {
		return nil, err
	}
	grouped := map[string]batchMemberRecord{}
	for _, member := range members {
		grouped[member.EventID] = member
	}
	calls := map[string]Call{}
	if len(callIDs) > 0 {
		var records []callRecord
		if err := q.Where("id IN ?", callIDs).Find(&records).Error; err != nil {
			return nil, err
		}
		for _, record := range records {
			calls[record.ID] = record.value()
		}
	}
	for _, row := range rows {
		e := row.value()
		if member, ok := grouped[e.ID]; ok {
			if member.Batch == nil {
				return nil, fmt.Errorf("missing batch %s", member.BatchID)
			}
			e.BatchID = member.BatchID
			e.BatchTitle = member.Batch.Title
			e.BatchDescription = member.Batch.Description
		}
		if row.CallID != "" {
			a, ok := calls[row.CallID]
			if !ok {
				return nil, fmt.Errorf("missing call %s", row.CallID)
			}
			e.Call = &a
		}
		out = append(out, e)
	}
	return out, nil
}
func (s *Store) Events(w string) ([]Event, error) { return events(s.db, w) }
func (s *Store) RepositoryEvents(rid string) ([]Event, error) {
	var ids []string
	if err := s.db.Model(&worktreeRecord{}).Where("repository_id = ?", rid).Order("root").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := []Event{}
	for _, id := range ids {
		es, err := s.Events(id)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}
func (s *Store) Event(w, id string) (Event, error) {
	es, err := s.Events(w)
	if err != nil {
		return Event{}, err
	}
	for _, e := range es {
		if e.ID == id {
			return e, nil
		}
	}
	return Event{}, gorm.ErrRecordNotFound
}
func (s *Store) CreateBatch(ctx context.Context, w, title, description string, ids []string) (b Batch, err error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(description) == "" || len(ids) == 0 {
		return b, invalid("batch requires title, description and patch IDs")
	}
	err = s.update(ctx, func(tx *gorm.DB) error {
		es, e := events(tx, w)
		if e != nil {
			return e
		}
		want := map[string]bool{}
		for _, id := range ids {
			if want[id] {
				return invalid("duplicate patch ID")
			}
			want[id] = true
		}
		var selected []Event
		for _, event := range es {
			if want[event.ID] {
				selected = append(selected, event)
			}
		}
		if len(selected) != len(ids) {
			return invalid("patch not found in this worktree")
		}
		first := selected[0]
		for i, event := range selected {
			if event.Kind != "tool_edit" || event.TurnID != first.TurnID || event.Segment != first.Segment || event.Commit != first.Commit || event.Sequence != first.Sequence+int64(i) {
				return invalid("batch members must be consecutive AI patches in one turn and commit boundary")
			}
			if event.BatchID != "" && event.BatchID != first.BatchID {
				return invalid("patch already belongs to a batch")
			}
		}
		if first.BatchID != "" {
			var count int64
			if e = tx.Model(&batchMemberRecord{}).Where("batch_id = ?", first.BatchID).Count(&count).Error; e != nil {
				return e
			}
			if count != int64(len(ids)) || first.BatchTitle != title || first.BatchDescription != description {
				return invalid("patches are already grouped; delete the old batch before replacing it")
			}
			b = Batch{ID: first.BatchID, Title: title, Description: description, Kind: "ai"}
			for _, event := range selected {
				if event.BatchID != b.ID {
					return invalid("partially grouped patches")
				}
				b.Members = append(b.Members, event.ID)
			}
			return nil
		}
		b = Batch{ID: ID(), Title: title, Description: description, Kind: "ai"}
		if e = tx.Create(&batchRecord{ID: b.ID, WorktreeID: w, Kind: b.Kind, Title: title, Description: description}).Error; e != nil {
			return e
		}
		for i, event := range selected {
			if event.BatchID != "" {
				return invalid("patch already belongs to a batch")
			}
			if e = tx.Create(&batchMemberRecord{BatchID: b.ID, EventID: event.ID, Position: i}).Error; e != nil {
				return e
			}
			b.Members = append(b.Members, event.ID)
		}
		return nil
	})
	return
}
func (s *Store) DeleteBatch(ctx context.Context, w, id string) error {
	return s.update(ctx, func(tx *gorm.DB) error {
		return tx.Where("id = ? AND worktree_id = ?", id, w).Delete(&batchRecord{}).Error
	})
}
func autoUnknown(tx *gorm.DB, w string) error {
	es, err := events(tx, w)
	if err != nil {
		return err
	}
	for i := 0; i < len(es); {
		if es[i].Kind != "unknown" {
			i++
			continue
		}
		j := i + 1
		for j < len(es) && es[j].Kind == "unknown" && es[j].Segment == es[i].Segment && es[j].Commit == es[i].Commit {
			j++
		}
		// Extend the existing automatic group, rebuilding only its mutable membership.
		id := ""
		old := map[string]bool{}
		for k := i; k < j; k++ {
			if es[k].BatchID != "" {
				old[es[k].BatchID] = true
				if id == "" {
					id = es[k].BatchID
				}
			}
		}
		if id == "" {
			id = ID()
		}
		keys := []string{}
		for key := range old {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err = tx.Where("id = ?", key).Delete(&batchRecord{}).Error; err != nil {
				return err
			}
		}
		if err = tx.Create(&batchRecord{ID: id, WorktreeID: w, Kind: "unknown", Title: "未知来源修改"}).Error; err != nil {
			return err
		}
		for k := i; k < j; k++ {
			if err = tx.Create(&batchMemberRecord{BatchID: id, EventID: es[k].ID, Position: k - i}).Error; err != nil {
				return err
			}
		}
		i = j
	}
	return nil
}
