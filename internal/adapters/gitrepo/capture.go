package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
)

var ErrCaptureChanged = errors.New("working tree changed during capture; retry observation")

func (r *Repository) gitInput(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	r.gitCalls.Add(1)
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = r.Root
	c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	c.Stdin = bytes.NewReader(input)
	var errout bytes.Buffer
	c.Stderr = &errout
	raw, e := c.Output()
	if e != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), errout.String(), e)
	}
	return raw, nil
}
func (r *Repository) captureCandidates(ctx context.Context, base domain.State) (map[string]bool, error) {
	out := map[string]bool{}
	// ls-files --modified deliberately over-reports stat changes without running
	// content conversion/diff filters just to discover candidate paths.
	args := []string{"-c", "core.fsmonitor=false", "-c", "core.ignoreStat=false", "-c", "core.trustctime=true", "-c", "core.checkStat=default", "ls-files", "--modified", "-z", "--"}
	raw, e := r.Git(ctx, args...)
	if e != nil {
		return nil, e
	}
	addPaths(out, raw)
	if ref, ok := base.Baseline(); ok && ref.Tree != "" {
		raw, e = r.Git(ctx, "diff-index", "--cached", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", ref.Tree, "--")
		if e != nil {
			return nil, e
		}
		addPaths(out, raw)
	} else {
		for p := range base.All() {
			out[p] = true
		}
	}
	return out, nil
}
func addPaths(out map[string]bool, raw []byte) {
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) > 0 {
			out[string(p)] = true
		}
	}
}
func (r *Repository) ignoredPaths(ctx context.Context, paths []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(paths) == 0 {
		return out, nil
	}
	var input bytes.Buffer
	for _, p := range paths {
		input.WriteString(p)
		input.WriteByte(0)
	}
	raw, e := r.gitInput(ctx, input.Bytes(), "check-ignore", "--no-index", "-z", "--stdin")
	if e != nil {
		var exit *exec.ExitError
		if !errors.As(e, &exit) || exit.ExitCode() != 1 {
			return nil, e
		}
	}
	addPaths(out, raw)
	return out, nil
}
func (r *Repository) captureAttributes(ctx context.Context, paths, candidates map[string]bool) (string, error) {
	var input bytes.Buffer
	// Stable order also makes the attribute fingerprint stable across processes.
	ordered := sortedPaths(paths)
	for _, p := range ordered {
		input.WriteString(p)
		input.WriteByte(0)
	}
	raw, e := r.gitInput(ctx, input.Bytes(), "check-attr", "-z", "--all", "--stdin")
	if e != nil {
		return "", e
	}
	records := bytes.Split(raw, []byte{0})
	for i := 0; i+2 < len(records); i += 3 {
		attr, value := string(records[i+1]), string(records[i+2])
		if value == "unset" || value == "unspecified" {
			continue
		}
		switch attr {
		case "filter", "text", "eol", "working-tree-encoding", "ident":
			candidates[string(records[i])] = true
		}
	}
	autocrlf := r.Config(ctx, "core.autocrlf")
	if autocrlf != "" && autocrlf != "false" {
		for p := range paths {
			candidates[p] = true
		}
	}
	return content.Hash(append(raw, []byte(autocrlf)...)), nil
}

// A bounded stable read detects replacements and edits while reading. It does not
// claim filesystem-wide atomicity; callers also verify the Git observation boundary.
func readStable(ctx context.Context, path string, info os.FileInfo) ([]byte, os.FileInfo, error) {
	for attempt := 0; attempt < 3; attempt++ {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		f, e := os.Open(path)
		if e != nil {
			return nil, nil, e
		}
		before, e := f.Stat()
		if e != nil {
			f.Close()
			return nil, nil, e
		}
		if !before.Mode().IsRegular() || !os.SameFile(info, before) {
			f.Close()
			return nil, nil, fmt.Errorf("%w: %s", ErrCaptureChanged, path)
		}
		raw, e := io.ReadAll(contextReader{ctx, f})
		after, se := f.Stat()
		ce := f.Close()
		if e != nil {
			return nil, nil, e
		}
		if se != nil {
			return nil, nil, se
		}
		if ce != nil {
			return nil, nil, ce
		}
		current, e := os.Lstat(path)
		if e != nil {
			return nil, nil, e
		}
		if os.SameFile(before, current) && stamp(before) == stamp(after) && stamp(after) == stamp(current) {
			return raw, current, nil
		}
		info = current
	}
	return nil, nil, fmt.Errorf("%w: %s", ErrCaptureChanged, path)
}

// Check from the root down before caching a directory as safe. Checking a child
// first could cache a path reached through an unsafe symlink ancestor.
func (r *Repository) blockedParents(ctx context.Context, p string, cache map[string]bool) (bool, error) {
	var parents []string
	for parent := filepath.Dir(p); parent != "."; parent = filepath.Dir(parent) {
		parents = append(parents, parent)
	}
	for i := len(parents) - 1; i >= 0; i-- {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		parent := parents[i]
		if safe, ok := cache[parent]; ok {
			if !safe {
				return true, nil
			}
			continue
		}
		info, e := os.Lstat(filepath.Join(r.Root, parent))
		if errors.Is(e, os.ErrNotExist) {
			cache[parent] = false
			return true, nil
		}
		if e != nil {
			return false, e
		}
		if !info.IsDir() {
			cache[parent] = false
			return true, nil
		}
		cache[parent] = true
	}
	return false, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
