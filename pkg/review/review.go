// Package review builds versioned review projections from captured changes.
package review

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	return s.resolve(ctx, source, target, false)
}

// CommitRange is independent of mutable branch names.
type CommitRange struct{ SourceOID, TargetOID, ComparisonMode string }
type BranchTracking struct{ Source, Target string }
type RefreshError struct {
	Code string
	Err  error
}

func (e *RefreshError) Error() string { return e.Err.Error() }
func (e *RefreshError) Unwrap() error { return e.Err }

const projectionAlgorithm = "2"

func inputSignature(source, target string, revision int64) string {
	return digest([]any{source, target, "merge_base", revision, projectionAlgorithm})
}

// ResolveTracking resolves both endpoints before returning a candidate range.
func (s *Service) ResolveTracking(ctx context.Context, t BranchTracking) (CommitRange, error) {
	source, err := s.Repo.ResolveBranch(ctx, t.Source)
	if err != nil {
		return CommitRange{}, &RefreshError{"source_branch_missing", err}
	}
	target, err := s.Repo.ResolveBranch(ctx, t.Target)
	if err != nil {
		return CommitRange{}, &RefreshError{"target_branch_missing", err}
	}
	return CommitRange{source, target, "merge_base"}, nil
}
func (s *Service) resolve(ctx context.Context, source, target string, allowEmpty bool) (p Preview, err error) {
	r, err := s.ResolveTracking(ctx, BranchTracking{source, target})
	if err != nil {
		return p, err
	}
	p, err = s.previewRange(ctx, r, allowEmpty)
	p.Source, p.Target = source, target
	return
}
func (s *Service) PreviewRange(ctx context.Context, r CommitRange) (Preview, error) {
	return s.previewRange(ctx, r, false)
}
func (s *Service) previewRange(ctx context.Context, r CommitRange, allowEmpty bool) (p Preview, err error) {
	if r.ComparisonMode != "" && r.ComparisonMode != "merge_base" {
		return p, fmt.Errorf("unsupported comparison mode")
	}
	if !gitrepo.ValidOID(r.SourceOID) || !gitrepo.ValidOID(r.TargetOID) {
		return p, fmt.Errorf("range requires full commit OIDs")
	}
	p.SourceOID, p.TargetOID = r.SourceOID, r.TargetOID
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
	raw, err = s.Repo.Git(ctx, "rev-list", "--reverse", "--topo-order", "--no-merges", p.TargetOID+".."+p.SourceOID)
	if err != nil {
		return p, err
	}
	p.Commits = strings.Fields(string(raw))
	if p.Commits == nil {
		p.Commits = []string{}
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
	if err == nil && len(p.Commits) == 0 && len(p.Diff) == 0 && !allowEmpty {
		err = fmt.Errorf("no new commits to review")
	}
	return
}
func (s *Service) Create(ctx context.Context, p Preview, title string) (r store.Review, err error) {
	if strings.TrimSpace(title) == "" {
		return r, fmt.Errorf("review title is required")
	}
	var current Preview
	if p.Source == "" && p.Target == "" {
		current, err = s.PreviewRange(ctx, CommitRange{p.SourceOID, p.TargetOID, "merge_base"})
	} else {
		current, err = s.Preview(ctx, p.Source, p.Target)
	}
	if err != nil {
		return r, err
	}
	if current.SourceOID != p.SourceOID || current.TargetOID != p.TargetOID {
		return r, fmt.Errorf("branches changed; preview the range again")
	}
	p = current
	r, err = s.build(ctx, current, title)
	if err != nil {
		return r, err
	}
	if len(r.Items) == 0 {
		return r, fmt.Errorf("no code changes to review")
	}
	if p.Source != "" {
		candidate, e := s.ResolveTracking(ctx, BranchTracking{p.Source, p.Target})
		if e != nil {
			return r, e
		}
		if candidate.SourceOID != p.SourceOID || candidate.TargetOID != p.TargetOID {
			return r, fmt.Errorf("branches changed; preview the range again")
		}
	}
	if err = s.Repo.Store.SaveReview(ctx, s.Worktree, &r); err != nil {
		return r, err
	}
	return s.Repo.Store.Review(s.Worktree.RepositoryID, r.ID)
}

// Refresh changes the published range only after both references and projection succeed.
func (s *Service) Refresh(ctx context.Context, id string) (store.Review, error) {
	attempt, err := s.Repo.Store.BeginRefresh(ctx, s.Worktree.RepositoryID, id)
	if err != nil {
		return store.Review{}, err
	}
	result, refreshErr := s.refresh(ctx, id)
	code, message := "ok", ""
	if refreshErr != nil {
		code = "build_failed"
		message = refreshErr.Error()
		var e *RefreshError
		if errors.As(refreshErr, &e) {
			code = e.Code
		}
	}
	if err = s.Repo.Store.FinishRefresh(ctx, s.Worktree.RepositoryID, id, attempt, code, message); err != nil {
		return result, err
	}
	if refreshErr != nil {
		return result, refreshErr
	}
	return s.Repo.Store.ReviewSummary(s.Worktree.RepositoryID, id)
}
func (s *Service) refresh(ctx context.Context, id string) (store.Review, error) {
	for attempt := 0; attempt < 3; attempt++ {
		previous, err := s.Repo.Store.ReviewSummary(s.Worktree.RepositoryID, id)
		if err != nil {
			return previous, err
		}
		source, target := previous.SourceOID, previous.TargetOID

		if previous.Source != "" || previous.Target != "" {
			candidate, e := s.ResolveTracking(ctx, BranchTracking{previous.Source, previous.Target})
			if e != nil {
				return previous, e
			}
			source, target = candidate.SourceOID, candidate.TargetOID
		}
		revision, err := s.Repo.Store.SourceRevision(ctx)
		if err != nil {
			return previous, err
		}
		if previous.InputSignature == inputSignature(source, target, revision) {
			return previous, nil
		}
		p, err := s.previewRange(ctx, CommitRange{source, target, "merge_base"}, true)
		if err != nil {
			return previous, &RefreshError{"range_unresolvable", err}
		}
		p.Source, p.Target = previous.Source, previous.Target
		next, err := s.build(ctx, p, previous.Title)
		if err != nil {
			return previous, err
		}
		for _, item := range previous.Items {
			if item.StableKey == "" && item.Kind == "batch" {
				next.Warning += "旧版审阅未保存来源批次身份，历史审批与评论保留，新批次需重新审阅。"
				break
			}
		}

		if previous.Source != "" {
			current, e := s.ResolveTracking(ctx, BranchTracking{previous.Source, previous.Target})
			if e != nil {
				return previous, e
			}
			if current.SourceOID != source || current.TargetOID != target {
				continue
			}
		}
		next.ID, next.Created = previous.ID, previous.Created
		if err = s.Repo.Store.PublishReview(ctx, s.Worktree, &next, previous.Version); errors.Is(err, store.ErrStaleView) {
			continue
		} else if err != nil {
			return previous, err
		}
		return next, nil
	}
	return store.Review{}, store.ErrStaleView
}

func digest(v any) string { raw, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func deltaDigest(before, after store.State) string {
	b, a := store.State{}, store.State{}
	for p, f := range before {
		if g, ok := after[p]; !ok || g != f {
			b[p] = f
		}
	}
	for p, f := range after {
		if g, ok := before[p]; !ok || g != f {
			a[p] = f
		}
	}
	return digest([]store.State{b, a})
}
func (s *Service) build(ctx context.Context, p Preview, title string) (r store.Review, err error) {
	r = store.Review{ID: store.ID(), RepositoryID: s.Worktree.RepositoryID, Title: title, Source: p.Source, Target: p.Target, SourceOID: p.SourceOID, TargetOID: p.TargetOID, Base: p.Base, Head: p.Head, Commits: p.Commits, Created: store.Now(), Status: "in_review", Integrity: "verified", Items: []store.ReviewItem{}}
	all, projected, warnings, revision, err := s.Repo.Store.ReviewInputs(ctx, s.Worktree.RepositoryID, p.Commits)
	if err != nil {
		return r, err
	}
	r.SourceRevision = revision
	r.InputSignature = inputSignature(p.SourceOID, p.TargetOID, revision)
	r.ComparisonMode = "merge_base"
	r.Closure = "verified"
	r.History = "complete"
	r.Diagnostics = []store.Diagnostic{}
	r.CheckSource = true
	r.Warning = strings.Join(warnings, " ")
	selected := map[string]bool{}
	for _, oid := range p.Commits {
		selected[oid] = true
	}
	pairCount := map[string]int{}
	for _, e := range all {
		if selected[e.Commit] && e.PairID != "" && projected[e.Commit] == e.Projected {
			pairCount[e.PairID]++
		}
	}
	events := []store.Event{}
	for _, oid := range p.Commits {
		var list []store.Event
		var worktree string
		for _, e := range all {
			if e.Commit != oid || projected[oid] != e.Projected {
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
		if e.Kind == "unknown" {
			r.History = "partial"
			r.Diagnostics = append(r.Diagnostics, store.Diagnostic{Code: "unknown_source", Stage: "history", EventID: e.ID, Message: "事件来源未知"})
		}
		j := i + 1
		if e.BatchID != "" {
			for j < len(events) && events[j].BatchID == e.BatchID {
				j++
			}
		}
		item := store.ReviewItem{Commit: e.Commit, SourceBatch: e.BatchID, ID: store.ID(), Kind: "unbatched_patch", Title: "未分批修改", Decision: "pending", Before: store.Clone(state), Events: []string{}}
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
			var conflicts []string
			state, conflicts, e1 = s.Repo.RebaseChange(ctx, state, before.State, after.State)
			if e1 != nil {
				return r, e1
			}
			if len(conflicts) > 0 {
				r.History = "partial"
				r.Diagnostics = append(r.Diagnostics, store.Diagnostic{Code: "projection_conflict", Stage: "replay", EventID: event.ID, Paths: conflicts, Message: "部分修改无法准确投影"})
				r.Warning += "无法准确投影的修改转为最终未知补齐：" + event.ID + " (" + strings.Join(conflicts, ", ") + ")。"
				item.Description += "\n部分路径不能准确投影，见原始事件与最终未知补齐：" + strings.Join(conflicts, ", ")
			}

			item.Events = append(item.Events, event.ID)
		}
		item.After = store.Clone(state)
		source := e.ID
		if e.BatchID != "" {
			source = e.BatchID
		}
		item.StableKey = e.Commit + "/" + item.Kind + "/" + source
		item.Digest = deltaDigest(item.Before, item.After)
		r.Items = append(r.Items, item)
		i = j
	}
	target, err := s.Repo.Tree(ctx, p.Head)
	if err != nil {
		return r, err
	}
	if !store.Equal(state, target) {
		r.Integrity = "completed_with_unknown"
		r.History = "partial"
		r.Diagnostics = append(r.Diagnostics, store.Diagnostic{Code: "unknown_completion", Stage: "integrity", Message: "最终状态包含未归属的净变化"})
		r.Items = append(r.Items, store.ReviewItem{StableKey: p.Head + "/integrity_unknown/" + deltaDigest(state, target), Commit: p.Head, Digest: deltaDigest(state, target), ID: store.ID(), Kind: "integrity_unknown", Title: "未知修改：最终差异补齐", Description: "Git 最终状态中没有被上述采集历史覆盖的净变化。", Decision: "pending", Before: store.Clone(state), After: target, Events: []string{}})
		state = target
	}
	if len(p.Commits) == 0 && len(r.Items) == 0 {
		r.Warning = "当前源/目标分支之间没有待审阅提交。"
	} else if len(events) == 0 {
		r.History = "unavailable"
		r.Warning += "未采集到数据，可能未安装 hook 或 hook 运行失败"
	} else {
		r.Warning += "已有采集记录不代表中间过程完整；最终状态已经验证。"
	}
	if !store.Equal(state, target) {
		return r, fmt.Errorf("review integrity verification failed")
	}
	identities := []string{}
	for _, item := range r.Items {
		identities = append(identities, item.StableKey+"/"+item.Digest+"/"+item.Title+"/"+item.Description)
	}
	r.ViewDigest = digest(struct {
		Source, Target, Base, Head string
		Commits, Items             []string
		History, Warning           string
		Diagnostics                []store.Diagnostic
	}{p.SourceOID, p.TargetOID, p.Base, p.Head, p.Commits, identities, r.History, r.Warning, r.Diagnostics})
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
	t.AnchorItem, err = s.CommentAnchor(ctx, r, t)
	if err != nil {
		return err
	}
	t.ID = store.ID()
	t.ReviewID = r.ID
	t.Hash = f.Hash
	return s.Repo.Store.AddThread(ctx, s.Worktree.RepositoryID, t, body, author, r.Version)
}
