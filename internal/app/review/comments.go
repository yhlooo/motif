package review

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"

	"github.com/yhlooo/motif/internal/adapters/gitrepo"
	"github.com/yhlooo/motif/internal/domain"
)

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type MappedThread struct {
	domain.Thread
	Ranges []LineRange `json:"mapped_ranges"`
}

func selectionIndices(r domain.Review, first, last string) (int, int, error) {
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
		return 0, 0, fmt.Errorf("invalid comment selection")
	}
	return a, b, nil
}
func (r *Service) mapStep(ctx context.Context, source, target domain.State, path string, lines []int) (string, []int, error) {
	from, ok := source.Lookup(path)
	if !ok || from.ContentRef == "" || from.OutOfScope {
		return path, nil, nil
	}
	to, exists := target.Lookup(path)
	if !exists {
		// Follow only an unambiguous exact-content rename, never a similar path.
		candidate := ""
		count := 0
		for p, f := range target.All() {
			if _, existed := source.Lookup(p); !existed && domain.SameFile(f, from) {
				candidate = p
				count++
			}
		}
		if count != 1 {
			return path, nil, nil
		}
		path = candidate
		to = target.Get(path)
	}
	if to.ContentRef == "" || to.OutOfScope {
		return path, nil, nil
	}
	if domain.SameFile(to, from) {
		return path, lines, nil
	}
	old, err := r.Repo.Store.Blob(from.ContentRef)
	if err != nil {
		return path, nil, err
	}
	new, err := r.Repo.Store.Blob(to.ContentRef)
	if err != nil {
		return path, nil, err
	}
	mapped := []int{}
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[j-1]+1 {
			j++
		}
		part, e := gitrepo.MapLines(ctx, old, new, lines[i], lines[j-1])
		if e != nil {
			return path, nil, e
		}
		mapped = append(mapped, part...)
		i = j
	}
	return path, mapped, nil
}
func (s *Service) CommentAnchor(ctx context.Context, r domain.Review, t domain.Thread) (string, error) {
	first, last, err := selectionIndices(r, t.First, t.Last)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(t.Path)
	if err != nil {
		return "", err
	}
	path := string(raw)
	lines := []int{}
	for n := t.Start; n <= t.End; n++ {
		lines = append(lines, n)
	}
	if t.Side == "new" {
		for i := last; i >= first; i-- {
			next, mapped, e := s.mapStep(ctx, r.Items[i].After, r.Items[i].Before, path, lines)
			if e != nil {
				return "", e
			}
			if len(mapped) == 0 {
				return r.Items[i].ID, nil
			}
			path, lines = next, mapped
		}
		return r.Items[last].ID, nil
	}
	for i := first; i <= last; i++ {
		next, mapped, e := s.mapStep(ctx, r.Items[i].Before, r.Items[i].After, path, lines)
		if e != nil {
			return "", e
		}
		if len(mapped) == 0 {
			return r.Items[i].ID, nil
		}
		path, lines = next, mapped
	}
	return r.Items[first].ID, nil
}
func (s *Service) MappedThreads(ctx context.Context, r domain.Review, first, last string) ([]MappedThread, error) {
	a, b, err := selectionIndices(r, first, last)
	if err != nil {
		return nil, err
	}
	threads, err := s.Repo.Store.Threads(s.Worktree.RepositoryID, r.ID)
	if err != nil {
		return nil, err
	}
	indices := map[string]int{}
	for i, item := range r.Items {
		indices[item.ID] = i
	}
	out := []MappedThread{}
	for _, thread := range threads {
		anchor := thread.AnchorItem
		if anchor == "" {
			anchor = thread.Last
			if thread.Side == "old" {
				anchor = thread.First
			}
		}
		index, active := indices[anchor]
		if !active || index < a || index > b {
			continue
		}
		raw, e := base64.StdEncoding.DecodeString(thread.Path)
		if e != nil {
			return nil, e
		}
		path := string(raw)
		points := []int{}
		for n := thread.Start; n <= thread.End; n++ {
			points = append(points, n)
		}
		// Original hash/range are immutable even when this identity is reprojected.
		anchorState := r.Items[index].After
		if thread.Side == "old" {
			anchorState = r.Items[index].Before
		}
		original := domain.From(map[string]domain.File{path: domain.File{Mode: "100644", ContentRef: thread.ContentRef}})
		originalID := thread.AfterID
		if thread.Side == "old" {
			originalID = thread.BeforeID
		}
		if originalID != "" {
			saved, e := s.Repo.Store.Snapshot(originalID)
			if e != nil {
				return nil, e
			}
			original = saved.State
		}
		path, points, e = s.mapStep(ctx, original, anchorState, path, points)
		if e != nil {
			return nil, e
		}
		if thread.Side == "new" {
			for i := index + 1; i <= b && len(points) > 0; i++ {
				path, points, e = s.mapStep(ctx, r.Items[i].Before, r.Items[i].After, path, points)
				if e != nil {
					return nil, e
				}
			}
		} else {
			for i := index - 1; i >= a && len(points) > 0; i-- {
				path, points, e = s.mapStep(ctx, r.Items[i].After, r.Items[i].Before, path, points)
				if e != nil {
					return nil, e
				}
			}
		}
		if len(points) == 0 {
			continue
		}
		sort.Ints(points)
		ranges := []LineRange{}
		for _, n := range points {
			if len(ranges) > 0 && ranges[len(ranges)-1].End+1 == n {
				ranges[len(ranges)-1].End = n
			} else {
				ranges = append(ranges, LineRange{n, n})
			}
		}
		thread.MappedStart = points[0]
		thread.MappedEnd = points[len(points)-1]
		thread.MappedPath = base64.StdEncoding.EncodeToString([]byte(path))
		out = append(out, MappedThread{thread, ranges})
	}
	return out, nil
}
