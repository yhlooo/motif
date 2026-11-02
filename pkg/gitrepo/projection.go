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
	"sort"
	"strconv"
	"strings"

	"github.com/yhlooo/motif/pkg/store"
)

// RebaseChange projects a recorded transition onto another state without changing
// Git's index or worktree. Conflicts preserve the input state for that path and
// are returned explicitly; the caller must account for them as unknown changes.
func (r *Repository) RebaseChange(ctx context.Context, current, before, after store.State) (store.State, []string, error) {
	out := store.Clone(current)
	paths := map[string]bool{}
	for p := range before {
		paths[p] = true
	}
	for p := range after {
		paths[p] = true
	}
	ordered := []string{}
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	failed := []string{}
	for _, p := range ordered {
		b, bok := before[p]
		a, aok := after[p]
		if bok == aok && b == a {
			continue
		}
		c, cok := current[p]
		if cok == bok && c == b {
			if aok {
				out[p] = a
			} else {
				delete(out, p)
			}
			continue
		}
		if cok == aok && c == a {
			continue
		}
		regular := func(f store.File) bool { return !f.OutOfScope && (f.Mode == "100644" || f.Mode == "100755") }
		if !bok || !aok || !cok || !regular(a) || !regular(b) || !regular(c) {
			failed = append(failed, store.DisplayPath(p))
			continue
		}
		if c.Mode != b.Mode && a.Mode != b.Mode && c.Mode != a.Mode {
			failed = append(failed, store.DisplayPath(p))
			continue
		}
		base, err := r.Store.Blob(b.Hash)
		if err != nil {
			return nil, nil, err
		}
		ours, err := r.Store.Blob(c.Hash)
		if err != nil {
			return nil, nil, err
		}
		theirs, err := r.Store.Blob(a.Hash)
		if err != nil {
			return nil, nil, err
		}
		if bytes.IndexByte(base, 0) >= 0 || bytes.IndexByte(ours, 0) >= 0 || bytes.IndexByte(theirs, 0) >= 0 {
			failed = append(failed, store.DisplayPath(p))
			continue
		}
		merged, clean, err := mergeBytes(ctx, ours, base, theirs)
		if err != nil {
			return nil, nil, err
		}
		if !clean {
			failed = append(failed, store.DisplayPath(p))
			continue
		}
		hash, err := r.Store.PutBlob(merged)
		if err != nil {
			return nil, nil, err
		}
		f := c
		f.Hash = hash
		if a.Mode != b.Mode {
			f.Mode = a.Mode
		}
		out[p] = f
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
