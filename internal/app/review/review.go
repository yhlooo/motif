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
	"github.com/yhlooo/motif/internal/app/workspace"
	"strings"

	"github.com/yhlooo/motif/internal/adapters/gitrepo"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/domain"
)

type Service struct {
	Repo     *workspace.Project
	Worktree domain.Worktree
}

// ErrBranchesChanged indicates that the preview no longer matches the tracking branches.
var ErrBranchesChanged = errors.New("branches changed; preview the range again")

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

const projectionAlgorithm = "immutable-state/1"

func inputSignature(repository, source, target string, revision int64) string {
	return digest([]any{repository, source, target, "merge_base", revision, projectionAlgorithm})
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
func (s *Service) Create(ctx context.Context, p Preview, title string) (r domain.Review, err error) {
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
		return r, ErrBranchesChanged
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
			return r, ErrBranchesChanged
		}
	}
	if err = s.Repo.Store.SaveReview(ctx, s.Worktree, &r); err != nil {
		return r, err
	}
	return s.Repo.Store.Review(s.Worktree.RepositoryID, r.ID)
}

// Refresh changes the published range only after both references and projection succeed.
func (s *Service) Refresh(ctx context.Context, id string) (domain.Review, error) {
	attempt, err := s.Repo.Store.BeginRefresh(ctx, s.Worktree.RepositoryID, id)
	if err != nil {
		return domain.Review{}, err
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
func (s *Service) refresh(ctx context.Context, id string) (domain.Review, error) {
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
		revision, err := s.Repo.Store.SourceRevision(ctx, s.Worktree.RepositoryID)
		if err != nil {
			return previous, err
		}
		if previous.InputSignature == inputSignature(s.Worktree.RepositoryID, source, target, revision) {
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
		if err = s.Repo.Store.PublishReview(ctx, s.Worktree, &next, previous.Version); errors.Is(err, sqlite.ErrStaleView) {
			continue
		} else if err != nil {
			return previous, err
		}
		return next, nil
	}
	return domain.Review{}, sqlite.ErrStaleView
}

func digest(v any) string { raw, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func (s *Service) build(ctx context.Context, p Preview, title string) (r domain.Review, err error) {
	r = domain.Review{ID: domain.ID(), RepositoryID: s.Worktree.RepositoryID, Title: title, Source: p.Source, Target: p.Target, SourceOID: p.SourceOID, TargetOID: p.TargetOID, Base: p.Base, Head: p.Head, Commits: p.Commits, Created: domain.Now(), Status: "in_review", Integrity: "verified", Items: []domain.ReviewItem{}}
	all, projected, warnings, revision, err := s.Repo.Store.ReviewInputs(ctx, s.Worktree.RepositoryID, p.Commits)
	if err != nil {
		return r, err
	}
	r.SourceRevision = revision
	r.InputSignature = inputSignature(s.Worktree.RepositoryID, p.SourceOID, p.TargetOID, revision)
	events, err := domain.SelectProjectionEvents(all, projected, p.Commits)
	if err != nil {
		return r, err
	}
	base, err := s.Repo.Tree(ctx, p.Base)
	if err != nil {
		return r, err
	}
	target, err := s.Repo.Tree(ctx, p.Head)
	if err != nil {
		return r, err
	}
	state := base
	steps := make([]domain.ProjectionStep, 0, len(events))
	for _, event := range events {
		before, e := s.Repo.Store.Snapshot(event.BeforeID)
		if e != nil {
			return r, e
		}
		after, e := s.Repo.Store.Snapshot(event.AfterID)
		if e != nil {
			return r, e
		}
		next, conflicts, e := s.Repo.RebaseChange(ctx, state, before.State, after.State)
		if e != nil {
			return r, e
		}
		steps = append(steps, domain.ProjectionStep{EventID: event.ID, Before: state, After: next, Conflicts: conflicts})
		state = next
	}
	return domain.AssembleProjection(domain.ProjectionInput{Review: r, Events: events, Steps: steps, Base: base, Target: target, Warnings: warnings})
}

func Selection(r domain.Review, first, last string) (domain.State, domain.State, error) {
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
		return domain.State{}, domain.State{}, fmt.Errorf("select a contiguous ordered range of review items")
	}
	return r.Items[a].Before, r.Items[b].After, nil
}
func (s *Service) AddThread(ctx context.Context, r domain.Review, t domain.Thread, body, author string) error {
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
	f, ok := state.Lookup(string(path))
	if !ok || f.ContentRef == "" || f.OutOfScope {
		return fmt.Errorf("comment path has no text on this side")
	}
	data, err := s.Repo.Store.Blob(f.ContentRef)
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
	t.ID = domain.ID()
	t.ReviewID = r.ID
	t.ContentRef = f.ContentRef
	return s.Repo.Store.AddThread(ctx, s.Worktree.RepositoryID, t, body, author, r.Version)
}
