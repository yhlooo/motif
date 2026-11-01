// Package review builds immutable review projections from captured changes.
package review

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/yhlooo/motif/pkg/gitrepo"
	"github.com/yhlooo/motif/pkg/store"
)

type Service struct {
	Repo     *gitrepo.Repository
	Worktree store.Worktree
}
type Preview struct {
	Source    string             `json:"source"`
	Target    string             `json:"target"`
	SourceOID string             `json:"source_oid"`
	TargetOID string             `json:"target_oid"`
	Base      string             `json:"base"`
	Head      string             `json:"head"`
	Commits   []string           `json:"commits"`
	Diff      []gitrepo.FileDiff `json:"diff"`
}

func (s *Service) Preview(ctx context.Context, source, target string) (p Preview, err error) {
	p.Source = source
	p.Target = target
	p.SourceOID, err = s.Repo.ResolveBranch(ctx, source)
	if err != nil {
		return p, err
	}
	p.TargetOID, err = s.Repo.ResolveBranch(ctx, target)
	if err != nil {
		return p, err
	}
	p.Head = p.SourceOID
	raw, err := s.Repo.Git(ctx, "merge-base", "--all", p.SourceOID, p.TargetOID)
	if err != nil {
		return p, err
	}
	bases := strings.Fields(string(raw))
	if len(bases) != 1 {
		return p, fmt.Errorf("review requires one unambiguous merge base")
	}
	p.Base = bases[0]
	raw, err = s.Repo.Git(ctx, "rev-list", "--reverse", "--topo-order", "--parents", p.Base+".."+p.Head)
	if err != nil {
		return p, err
	}
	p.Commits = []string{}
	parent := p.Base
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || f[1] != parent {
			return p, fmt.Errorf("MVP supports linear commit ranges; merge histories need an explicit comparison parent")
		}
		p.Commits = append(p.Commits, f[0])
		parent = f[0]
	}
	if len(p.Commits) == 0 {
		return p, fmt.Errorf("no new commits to review")
	}
	before, err := s.Repo.Tree(ctx, p.Base)
	if err != nil {
		return p, err
	}
	after, err := s.Repo.Tree(ctx, p.Head)
	if err != nil {
		return p, err
	}
	p.Diff, err = s.Repo.Diff(ctx, before, after)
	return
}
func (s *Service) Create(ctx context.Context, p Preview, title string) (r store.Review, err error) {
	if strings.TrimSpace(title) == "" {
		return r, fmt.Errorf("review title is required")
	}
	current, err := s.Preview(ctx, p.Source, p.Target)
	if err != nil {
		return r, err
	}
	if current.SourceOID != p.SourceOID || current.TargetOID != p.TargetOID {
		return r, fmt.Errorf("branches changed; preview the range again")
	}
	p = current
	r = store.Review{ID: store.ID(), RepositoryID: s.Worktree.RepositoryID, Title: title, Source: p.Source, Target: p.Target, SourceOID: p.SourceOID, TargetOID: p.TargetOID, Base: p.Base, Head: p.Head, Commits: p.Commits, Created: store.Now(), Status: "in_review", Integrity: "verified", Items: []store.ReviewItem{}}
	all, err := s.Repo.Store.RepositoryEvents(s.Worktree.RepositoryID)
	if err != nil {
		return r, err
	}
	selected := map[string]bool{}
	for _, oid := range p.Commits {
		selected[oid] = true
	}
	pairCount := map[string]int{}
	for _, e := range all {
		if selected[e.Commit] && e.PairID != "" {
			pairCount[e.PairID]++
		}
	}
	events := []store.Event{}
	for _, oid := range p.Commits {
		var list []store.Event
		var worktree string
		for _, e := range all {
			if e.Commit != oid {
				continue
			}
			if worktree != "" && worktree != e.WorktreeID {
				return r, fmt.Errorf("multiple captured worktrees claim commit %s; projection is ambiguous", oid)
			}
			worktree = e.WorktreeID
			if e.PairID != "" && pairCount[e.PairID] == 2 {
				continue
			}
			list = append(list, e)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Sequence < list[j].Sequence })
		events = append(events, list...)
	}
	state, err := s.Repo.Tree(ctx, p.Base)
	if err != nil {
		return r, err
	}
	for i := 0; i < len(events); {
		e := events[i]
		j := i + 1
		if e.BatchID != "" {
			for j < len(events) && events[j].BatchID == e.BatchID {
				j++
			}
		}
		item := store.ReviewItem{ID: store.ID(), Kind: "unbatched_patch", Title: "未分批修改", Decision: "pending", Before: store.Clone(state), Events: []string{}}
		if e.BatchID != "" {
			item.Kind = "batch"
			item.Title = e.BatchTitle
			item.Description = e.BatchDescription
		} else if e.Kind == "virtual_close" {
			item.Kind = "commit_virtual"
			item.Title = "revert 部分暂不提交内容"
		} else if e.Kind == "virtual_carry" {
			item.Kind = "commit_virtual"
			item.Title = "之前暂存变更"
		}
		for k := i; k < j; k++ {
			event := events[k]
			before, e1 := s.Repo.Store.Snapshot(event.BeforeID)
			if e1 != nil {
				return r, e1
			}
			after, e1 := s.Repo.Store.Snapshot(event.AfterID)
			if e1 != nil {
				return r, e1
			}
			state, e1 = store.Apply(state, before.State, after.State)
			if e1 != nil {
				return r, fmt.Errorf("incomplete captured history at patch %s: %w", event.ID, e1)
			}
			item.Events = append(item.Events, event.ID)
		}
		item.After = store.Clone(state)
		r.Items = append(r.Items, item)
		i = j
	}
	target, err := s.Repo.Tree(ctx, p.Head)
	if err != nil {
		return r, err
	}
	if !store.Equal(state, target) {
		r.Integrity = "completed_with_unknown"
		r.Items = append(r.Items, store.ReviewItem{ID: store.ID(), Kind: "integrity_unknown", Title: "未知修改：最终差异补齐", Description: "Git 最终状态中没有被上述采集历史覆盖的净变化。", Decision: "pending", Before: store.Clone(state), After: target, Events: []string{}})
		state = target
	}
	if len(events) == 0 {
		r.Warning = "未采集到数据，可能未安装 hook 或 hook 运行失败"
	} else {
		r.Warning = "已有采集记录不代表中间过程完整；最终状态已经验证。"
	}
	if !store.Equal(state, target) {
		return r, fmt.Errorf("review integrity verification failed")
	}
	// A sequence that cancels out is reviewable even if its final diff is empty.
	if len(r.Items) == 0 {
		return r, fmt.Errorf("no code changes to review")
	}
	if err = s.Repo.Store.SaveReview(ctx, s.Worktree, &r); err != nil {
		return r, err
	}
	return r, nil
}
func Selection(r store.Review, first, last string) (store.State, store.State, error) {
	a, b := -1, -1
	for i, item := range r.Items {
		if item.ID == first {
			a = i
		}
		if item.ID == last {
			b = i
		}
	}
	if a < 0 || b < a {
		return nil, nil, fmt.Errorf("select a contiguous ordered range of review items")
	}
	for i := a; i < b; i++ {
		if !store.Equal(r.Items[i].After, r.Items[i+1].Before) {
			return nil, nil, fmt.Errorf("review state chain is discontinuous")
		}
	}
	return r.Items[a].Before, r.Items[b].After, nil
}
func (s *Service) AddThread(ctx context.Context, r store.Review, t store.Thread, body, author string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is required")
	}
	if t.Side != "old" && t.Side != "new" {
		return fmt.Errorf("invalid comment side")
	}
	before, after, err := Selection(r, t.First, t.Last)
	if err != nil {
		return err
	}
	path, err := base64.StdEncoding.DecodeString(t.Path)
	if err != nil {
		return fmt.Errorf("invalid path encoding")
	}
	state := after
	if t.Side == "old" {
		state = before
	}
	f, ok := state[string(path)]
	if !ok || f.Hash == "" || f.OutOfScope {
		return fmt.Errorf("comment path has no text on this side")
	}
	data, err := s.Repo.Store.Blob(f.Hash)
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte{0}) {
		return fmt.Errorf("binary files cannot have line comments")
	}
	count := bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		count++
	}
	if t.Start < 1 || t.End < t.Start || t.End > count {
		return fmt.Errorf("comment line range is outside the selected file")
	}
	t.ID = store.ID()
	t.ReviewID = r.ID
	t.Hash = f.Hash
	return s.Repo.Store.AddThread(ctx, s.Worktree.RepositoryID, t, body, author)
}
