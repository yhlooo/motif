package capture

import (
	"bufio"
	"context"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/domain"
)

func topology(ctx context.Context, r *workspace.Project, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := append([]string{"rev-list", "--reverse", "--topo-order"}, ids...)
	raw, err := r.Git(ctx, args...)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	out := []string{}
	for _, id := range strings.Fields(string(raw)) {
		if wanted[id] {
			out = append(out, id)
		}
	}
	return out, nil
}
func Rewrite(ctx context.Context, r *workspace.Project, w domain.Worktree, kind string, input io.Reader) error {
	if kind != "amend" && kind != "rebase" {
		return fmt.Errorf("unsupported rewrite kind %s", kind)
	}
	mappings := []domain.RewriteMapping{}
	oldIDs, newIDs := []string{}, []string{}
	newOlds := map[string]map[string]bool{}
	oldTargets := map[string]map[string]bool{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) != 2 {
			return fmt.Errorf("invalid post-rewrite mapping")
		}
		if f[0] == f[1] {
			continue
		}
		if _, err := r.Tree(ctx, f[0]); err != nil {
			return err
		}
		if _, err := r.Tree(ctx, f[1]); err != nil {
			return err
		}
		mappings = append(mappings, domain.RewriteMapping{Old: f[0], New: f[1]})
		oldIDs = append(oldIDs, f[0])
		newIDs = append(newIDs, f[1])
		if newOlds[f[1]] == nil {
			newOlds[f[1]] = map[string]bool{}
		}
		newOlds[f[1]][f[0]] = true
		if oldTargets[f[0]] == nil {
			oldTargets[f[0]] = map[string]bool{}
		}
		oldTargets[f[0]][f[1]] = true
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(mappings) == 0 {
		return nil
	}
	existing, err := r.Store.ProjectedCommits(w.RepositoryID)
	if err != nil {
		return err
	}
	allDone := true
	for _, id := range newIDs {
		allDone = allDone && existing[id]
	}
	if allDone {
		return nil
	}
	cursor, err := r.Store.Cursor(w.ID)
	if err != nil {
		return err
	}
	head, err := r.Head(ctx)
	if err != nil {
		return err
	}
	var workspace domain.State
	pending := []string{}
	if kind == "amend" {
		base, err := r.Tree(ctx, cursor.Base)
		if err != nil {
			return err
		}
		workspace, err = r.Worktree(ctx, base)
		if err != nil {
			return err
		}
		o := domain.Observation{Worktree: w, Base: cursor.Base, BaseState: base, State: workspace, Trigger: "rewrite_observation", ExpectedVersion: cursor.Version, CallerSession: os.Getenv("CODEX_THREAD_ID"), CallerID: os.Getenv("CODEX_TOOL_CALL_ID")}
		if err = r.Store.Observe(ctx, o); err != nil {
			return err
		}
		cursor, err = r.Store.Cursor(w.ID)
		if err != nil {
			return err
		}
	}
	events, _, _, sourceRevision, err := r.Store.ReviewInputs(ctx, w.RepositoryID, oldIDs)
	if err != nil {
		return err
	}
	oldOrder, err := topology(ctx, r, oldIDs)
	if err != nil {
		return err
	}
	newOrder, err := topology(ctx, r, newIDs)
	if err != nil {
		return err
	}
	pairs := []string{}
	for _, m := range mappings {
		pairs = append(pairs, m.Old+":"+m.New)
	}
	sort.Strings(pairs)
	operation := strings.Join(pairs, ";")
	prepared := []domain.CommitProjection{}
	resumePair := ""
	for _, newOID := range newOrder {
		if existing[newOID] {
			continue
		}
		parent, err := r.Parent(ctx, newOID)
		if err != nil {
			return err
		}
		state, err := r.Tree(ctx, parent)
		if err != nil {
			return err
		}
		target, err := r.Tree(ctx, newOID)
		if err != nil {
			return err
		}
		projection := domain.CommitProjection{Commit: newOID, Parent: parent}
		source := []domain.Event{}
		for _, old := range oldOrder {
			if !newOlds[newOID][old] {
				continue
			}
			for _, event := range events {
				if event.Commit == old && event.Projected == existing[old] {
					source = append(source, event)
				}
			}
		}
		if kind == "amend" && len(mappings) == 1 {
			for _, event := range events {
				if event.WorktreeID == w.ID && event.Segment == cursor.Segment && event.Commit == "" && !event.Projected {
					source = append(source, event)
					pending = append(pending, event.ID)
				}
			}
		}
		for _, event := range source {
			before, err := r.Store.Snapshot(event.BeforeID)
			if err != nil {
				return err
			}
			after, err := r.Store.Snapshot(event.AfterID)
			if err != nil {
				return err
			}
			next, conflicts, err := r.RebaseChange(ctx, state, before.State, after.State)
			if err != nil {
				return err
			}
			if len(oldTargets[event.Commit]) > 1 {
				// A split may inherit only independently verified file transitions. Same-file
				// hunk splits that cannot be proved remain unknown instead of duplicating intent.
				for p, f := range next.All() {
					previous, had := state.Lookup(p)
					if had && domain.SameFile(previous, f) {
						continue
					}
					wanted, ok := target.Lookup(p)
					if !ok || !domain.SameFile(wanted, f) {
						if had {
							next = next.Set(p, previous)
						} else {
							next = next.Delete(p)
						}
						conflicts = append(conflicts, domain.DisplayPath(p))
					}
				}
				for p, f := range state.All() {
					if _, ok := next.Lookup(p); ok {
						continue
					}
					if _, still := target.Lookup(p); still {
						next = next.Set(p, f)
						conflicts = append(conflicts, domain.DisplayPath(p))
					}
				}
			}
			if len(conflicts) > 0 {
				projection.Warning += "无法准确映射 " + event.ID + ": " + strings.Join(conflicts, ", ") + "。"
			}
			if !domain.Equal(state, next) {
				kind := event.Kind
				pair := ""
				if kind == "virtual_close" || kind == "virtual_carry" {
					pair = domain.ProjectionBatchID(operation, event.PairID)
				}
				projection.Changes = append(projection.Changes, domain.ProjectedChange{Origin: event, Before: state, After: next, Kind: kind, PairID: pair})
				state = next
			}
		}
		if kind == "amend" && newOID == head && !domain.Equal(state, workspace) {
			projection.Changes = append(projection.Changes, domain.ProjectedChange{Before: state, After: workspace, Kind: "unknown"})
			state = workspace
		}
		if !domain.Equal(state, target) {
			eventKind, pair := "unknown", ""
			if kind == "amend" && newOID == head && !domain.Equal(workspace, target) {
				eventKind = "virtual_close"
				pair = domain.ID()
				resumePair = pair
			}
			projection.Changes = append(projection.Changes, domain.ProjectedChange{Before: state, After: target, Kind: eventKind, PairID: pair})
			state = target
		}
		if !domain.Equal(state, target) {
			return fmt.Errorf("rewrite projection does not close commit %s", newOID)
		}
		prepared = append(prepared, projection)
	}
	type location struct{ p, c int }
	virtual := map[string][]location{}
	for i := range prepared {
		for j, change := range prepared[i].Changes {
			if change.Origin.PairID != "" && (change.Kind == "virtual_close" || change.Kind == "virtual_carry") {
				virtual[change.PairID] = append(virtual[change.PairID], location{i, j})
			}
		}
	}
	for _, locations := range virtual {
		valid := false
		if len(locations) == 2 {
			a := prepared[locations[0].p].Changes[locations[0].c]
			b := prepared[locations[1].p].Changes[locations[1].c]
			valid = a.Kind != b.Kind && inverseChanges(a.Before, a.After, b.Before, b.After)
		}
		if !valid {
			for _, at := range locations {
				prepared[at.p].Changes[at.c].Kind = "unknown"
				prepared[at.p].Changes[at.c].PairID = ""
				prepared[at.p].Warning += "提交闭合事件无法验证成对逆变换，按未知保留。"
			}
		}
	}
	baseline, err := r.Tree(ctx, head)
	if err != nil {
		return err
	}
	if kind != "amend" {
		workspace, err = r.Worktree(ctx, baseline)
		if err != nil {
			return err
		}
	}
	resume := domain.Observation{Worktree: w, Base: head, BaseState: baseline, State: workspace, Trigger: "rewrite_resume", Reset: true, KeepCall: true, ResumePair: resumePair, ExpectedVersion: cursor.Version, CallerSession: os.Getenv("CODEX_THREAD_ID"), CallerID: os.Getenv("CODEX_TOOL_CALL_ID")}
	return r.Store.SaveRewrite(ctx, domain.RewritePlan{Worktree: w, Kind: kind, Mappings: mappings, Projections: prepared, Pending: pending, ExpectedCursor: cursor.Version, SourceRevision: sourceRevision, Resume: resume})
}

// References records only committed ref notifications; it does not assume that
// the worktree is already settled (notably during reset/rebase).
func References(ctx context.Context, r *workspace.Project, w domain.Worktree, phase string, input io.Reader) error {
	if phase == "preparing" || phase == "prepared" || phase == "aborted" {
		return nil
	}
	if phase != "committed" {
		return fmt.Errorf("invalid reference transaction phase")
	}
	scanner := bufio.NewScanner(input)
	updates := []sqlite.RefTransition{}
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) != 3 {
			return fmt.Errorf("invalid reference transaction")
		}
		updates = append(updates, sqlite.RefTransition{Old: f[0], New: f[1], Ref: f[2]})
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return r.Store.RecordReferences(ctx, w.RepositoryID, updates)
}

func inverseChanges(a, b, c, d domain.State) bool {
	paths := map[string]bool{}
	for p := range a.All() {
		paths[p] = true
	}
	for p := range b.All() {
		paths[p] = true
	}
	for p := range c.All() {
		paths[p] = true
	}
	for p := range d.All() {
		paths[p] = true
	}
	for p := range paths {
		av, ao := a.Lookup(p)
		bv, bo := b.Lookup(p)
		cv, co := c.Lookup(p)
		dv, do := d.Lookup(p)
		first := ao != bo || !domain.SameFile(av, bv)
		second := co != do || cv != dv
		if first != second {
			return false
		}
		if first && (ao != do || bo != co || !domain.SameFile(av, dv) || !domain.SameFile(bv, cv)) {
			return false
		}
	}
	return true
}
