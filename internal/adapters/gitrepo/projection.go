package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
)

// RebaseChange projects a recorded transition onto another state without changing
// Git's index or worktree. Conflicts preserve the input state for that path and
// are returned explicitly; the caller must account for them as unknown changes.
func (r *Repository) RebaseChange(ctx context.Context, current, before, after domain.State) (domain.State, []string, error) {
	out := domain.Clone(current)
	ordered := []string{}
	for p := range domain.ChangedPaths(before, after) {
		ordered = append(ordered, p)
	}
	failed := []string{}
	for _, p := range ordered {
		b, bok := before.Lookup(p)
		a, aok := after.Lookup(p)
		if bok == aok && domain.SameFile(b, a) {
			continue
		}
		c, cok := current.Lookup(p)
		if cok == bok && domain.SameFile(c, b) {
			if aok {
				out = out.Set(p, a)
			} else {
				out = out.Delete(p)
			}
			continue
		}
		if cok == aok && domain.SameFile(c, a) {
			continue
		}
		regular := func(f domain.File) bool { return !f.OutOfScope && (f.Mode == "100644" || f.Mode == "100755") }
		if !bok || !aok || !cok || !regular(a) || !regular(b) || !regular(c) {
			failed = append(failed, domain.DisplayPath(p))
			continue
		}
		if c.Mode != b.Mode && a.Mode != b.Mode && c.Mode != a.Mode {
			failed = append(failed, domain.DisplayPath(p))
			continue
		}
		base, err := r.Objects.Read(ctx, b.ContentRef)
		if err != nil {
			return domain.State{}, nil, err
		}
		ours, err := r.Objects.Read(ctx, c.ContentRef)
		if err != nil {
			return domain.State{}, nil, err
		}
		theirs, err := r.Objects.Read(ctx, a.ContentRef)
		if err != nil {
			return domain.State{}, nil, err
		}
		if bytes.IndexByte(base, 0) >= 0 || bytes.IndexByte(ours, 0) >= 0 || bytes.IndexByte(theirs, 0) >= 0 {
			failed = append(failed, domain.DisplayPath(p))
			continue
		}
		merged, clean, err := mergeBytes(ctx, ours, base, theirs)
		if err != nil {
			return domain.State{}, nil, err
		}
		if !clean {
			failed = append(failed, domain.DisplayPath(p))
			continue
		}
		hash, err := r.Objects.Put(merged)
		if err != nil {
			return domain.State{}, nil, err
		}
		f := c
		f.ContentRef = hash
		f.ContentID = content.GitIdentity(merged, r.ObjectFormat)
		if a.Mode != b.Mode {
			f.Mode = a.Mode
		}
		out = out.Set(p, f)
	}
	return out, failed, nil
}
func mergeBytes(ctx context.Context, ours, base, theirs []byte) ([]byte, bool, error) {
	dir, err := os.MkdirTemp("", "motif-merge-")
	if err != nil {
		return nil, false, err
	}
	defer os.RemoveAll(dir)
	for name, data := range map[string][]byte{"ours": ours, "base": base, "theirs": theirs} {
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return nil, false, err
		}
	}
	cmd := exec.CommandContext(ctx, "git", "merge-file", "-p", "--", filepath.Join(dir, "ours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs"))
	raw, err := cmd.Output()
	if err == nil {
		return raw, true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() > 0 && exit.ExitCode() <= 127 {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("merge-file: %w", err)
}
func (r *Repository) Parent(ctx context.Context, oid string) (string, error) {
	raw, err := r.Git(ctx, "rev-list", "--parents", "-n", "1", oid)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 1 {
		return "", nil
	}
	if len(fields) < 2 {
		return "", fmt.Errorf("invalid commit %s", oid)
	}
	return fields[1], nil
}

var hunkPattern = regexp.MustCompile(`(?m)^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// MapLines returns only surviving, unchanged source lines. A deleted/replaced
// line is never attached to a nearby similar line. The original anchor is kept.
func mapLinesUncached(ctx context.Context, source, target []byte, start, end int) ([]int, error) {
	if start < 1 || end < start {
		return nil, fmt.Errorf("invalid line range")
	}
	if bytes.IndexByte(source, 0) >= 0 || bytes.IndexByte(target, 0) >= 0 {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "motif-lines-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	a, b := filepath.Join(dir, "before"), filepath.Join(dir, "after")
	if err = os.WriteFile(a, source, 0600); err != nil {
		return nil, err
	}
	if err = os.WriteFile(b, target, 0600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--unified=0", "--", a, b)
	raw, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, err
		}
	}
	type change struct{ old, count, newCount int }
	var changes []change
	for _, h := range hunkPattern.FindAllSubmatch(raw, -1) {
		old, _ := strconv.Atoi(string(h[1]))
		count := 1
		if len(h[2]) > 0 {
			count, _ = strconv.Atoi(string(h[2]))
		}
		newCount := 1
		if len(h[4]) > 0 {
			newCount, _ = strconv.Atoi(string(h[4]))
		}
		changes = append(changes, change{old, count, newCount})
	}
	count := bytes.Count(source, []byte{'\n'})
	if len(source) > 0 && source[len(source)-1] != '\n' {
		count++
	}
	mapped := []int{}
	for line := start; line <= end && line <= count; line++ {
		delta := 0
		survives := true
		for _, c := range changes {
			if c.count == 0 {
				if line > c.old {
					delta += c.newCount
				}
				continue
			}
			if line < c.old {
				break
			}
			if line < c.old+c.count {
				survives = false
				break
			}
			delta += c.newCount - c.count
		}
		if survives {
			mapped = append(mapped, line+delta)
		}
	}
	return mapped, nil
}
