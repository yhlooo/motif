package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yhlooo/motif/internal/domain"
	"gorm.io/gorm"
)

var ErrInvalidArgument = errors.New("invalid argument")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, fmt.Sprintf(format, args...))
}

func events(q *gorm.DB, w string) ([]domain.Event, error) {
	var rows []eventRecord
	if err := q.Where("worktree_id = ?", w).Order("projected").Order("sequence").Find(&rows).Error; err != nil {
		return nil, err
	}
	var associations []commitChangeRecord
	if len(rows) > 0 {
		ids := []string{}
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		if e := q.Where("event_id IN ?", ids).Find(&associations).Error; e != nil {
			return nil, e
		}
	}
	commits := map[string]string{}
	for _, a := range associations {
		commits[a.EventID] = a.Commit
	}
	origins := map[string][]string{}
	if len(rows) > 0 {
		ids := []string{}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		var edges []derivationRecord
		if e := q.Where("change_id IN ?", ids).Order("origin_id").Find(&edges).Error; e != nil {
			return nil, e
		}
		for _, edge := range edges {
			origins[edge.ChangeID] = append(origins[edge.ChangeID], edge.OriginID)
		}
	}
	out := make([]domain.Event, 0, len(rows))
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
	calls := map[string]domain.Call{}
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
		e.Origins = origins[row.ID]
		e.Commit = commits[row.ID]
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
func (s *Store) Events(w string) ([]domain.Event, error) { return events(s.db, w) }
func (s *Store) RepositoryEvents(rid string) ([]domain.Event, error) {
	var ids []string
	if err := s.db.Model(&worktreeRecord{}).Where("repository_id = ?", rid).Order("root").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	out := []domain.Event{}
	for _, id := range ids {
		es, err := s.Events(id)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}
func (s *Store) Event(w, id string) (domain.Event, error) {
	es, err := s.Events(w)
	if err != nil {
		return domain.Event{}, err
	}
	for _, e := range es {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.Event{}, gorm.ErrRecordNotFound
}
func (s *Store) CreateBatch(ctx context.Context, w, title, description string, ids []string) (b domain.Batch, err error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(description) == "" || len(ids) == 0 {
		return b, invalid("batch requires title, description and patch IDs")
	}
	err = s.sourceUpdate(ctx, func(tx *gorm.DB) error {
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
		var selected []domain.Event
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
			b = domain.Batch{ID: first.BatchID, Title: title, Description: description, Kind: "ai"}
			for _, event := range selected {
				if event.BatchID != b.ID {
					return invalid("partially grouped patches")
				}
				b.Members = append(b.Members, event.ID)
			}
			return nil
		}
		b = domain.Batch{ID: domain.ID(), Title: title, Description: description, Kind: "ai"}
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
		if e = mirrorBatch(tx, w, b, selected); e != nil {
			return e
		}
		return bumpSource(tx, w)
	})
	return
}
func (s *Store) DeleteBatch(ctx context.Context, w, id string) error {
	return s.sourceUpdate(ctx, func(tx *gorm.DB) error {
		r := tx.Where("id = ? AND worktree_id = ?", id, w).Delete(&batchRecord{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected > 0 {
			return bumpSource(tx, w)
		}
		return nil
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
		same := es[i].BatchID != ""
		for k := i; k < j; k++ {
			same = same && es[k].BatchID == es[i].BatchID
		}
		if same {
			var n int64
			if e := tx.Model(&batchMemberRecord{}).Where("batch_id = ?", es[i].BatchID).Count(&n).Error; e != nil {
				return e
			}
			if n == int64(j-i) {
				i = j
				continue
			}
		}
		if e := bumpSource(tx, w); e != nil {
			return e
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
			id = domain.ID()
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

func mirrorBatch(tx *gorm.DB, w string, b domain.Batch, source []domain.Event) error {
	seeds := []string{}
	for _, e := range source {
		seeds = append(seeds, e.ID)
	}
	derived := []eventRecord{}
	seen := map[string]bool{}
	for len(seeds) > 0 {
		var rows []eventRecord
		if err := tx.Where("projected = ? AND id IN (?)", true, tx.Model(&derivationRecord{}).Select("change_id").Where("origin_id IN ?", seeds)).Order("sequence").Find(&rows).Error; err != nil {
			return err
		}
		seeds = nil
		for n := range rows {
			var a commitChangeRecord
			if e := tx.Where("event_id = ?", rows[n].ID).Take(&a).Error; e != nil {
				return e
			}
			rows[n].FinalCommit = a.Commit
		}
		for _, row := range rows {
			if seen[row.ID] {
				continue
			}
			seen[row.ID] = true
			derived = append(derived, row)
			seeds = append(seeds, row.ID)
		}
	}
	sort.Slice(derived, func(i, j int) bool {
		if derived[i].FinalCommit != derived[j].FinalCommit {
			return derived[i].FinalCommit < derived[j].FinalCommit
		}
		return derived[i].Sequence < derived[j].Sequence
	})
	for i := 0; i < len(derived); {
		var used int64
		if err := tx.Model(&batchMemberRecord{}).Where("event_id = ?", derived[i].ID).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			i++
			continue
		}
		j := i + 1
		for j < len(derived) && derived[j].FinalCommit == derived[i].FinalCommit && derived[j].Sequence == derived[j-1].Sequence+1 {
			var n int64
			if err := tx.Model(&batchMemberRecord{}).Where("event_id = ?", derived[j].ID).Count(&n).Error; err != nil {
				return err
			}
			if n > 0 {
				break
			}
			j++
		}
		id := domain.ProjectionBatchID(derived[i].FinalCommit, b.ID+"/"+derived[i].ID)
		record := batchRecord{ID: id, WorktreeID: derived[i].WorktreeID, Kind: b.Kind, Title: b.Title, Description: b.Description, OriginBatch: b.ID}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		for k := i; k < j; k++ {
			if err := tx.Create(&batchMemberRecord{BatchID: id, EventID: derived[k].ID, Position: k - i}).Error; err != nil {
				return err
			}
		}
		i = j
	}
	return nil
}
