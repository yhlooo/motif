// Package gitrepo reads repository state without changing the user's index.
package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yhlooo/motif/pkg/store"
)

type Repository struct {
	Root, GitDir, CommonDir string
	Store                   *store.Store
	treeMu                  sync.Mutex
	trees                   map[string]store.State
	treeEntries             int
	treeLoads               atomic.Int64
	gitCalls                atomic.Int64
	blobBytes               atomic.Int64
	diffCache               boundedCache
}

func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	c.Stderr = &stderr
	b, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return b, nil
}
func Open(ctx context.Context, dir string, s *store.Store) (*Repository, error) {
	root, err := run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	r := &Repository{Root: strings.TrimSpace(string(root)), Store: s}
	g, err := r.Git(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	r.GitDir = strings.TrimSpace(string(g))
	g, err = r.Git(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	r.CommonDir = strings.TrimSpace(string(g))
	if rel, e := filepath.Rel(r.Root, s.Dir); e != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("Motif data directory must be outside the worktree")
	}
	r.Store.SetBaseReader(r.CommonDir, r.Tree)
	return r, nil
}
func (r *Repository) Git(ctx context.Context, args ...string) ([]byte, error) {
	r.gitCalls.Add(1)
	return run(ctx, r.Root, args...)
}
func (r *Repository) Identity(ctx context.Context) (store.Worktree, error) {
	return r.Store.Register(ctx, r.CommonDir, r.Root, r.GitDir)
}
func (r *Repository) Head(ctx context.Context) (string, error) {
	b, e := r.Git(ctx, "rev-parse", "--verify", "HEAD")
	if e == nil {
		return strings.TrimSpace(string(b)), nil
	}
	b, e = r.Git(ctx, "symbolic-ref", "-q", "HEAD")
	if e != nil {
		return "", e
	}
	if len(b) == 0 {
		return "", fmt.Errorf("cannot resolve HEAD")
	}
	return "", nil
}
func (r *Repository) ResolveBranch(ctx context.Context, name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "\x00\r\n") {
		return "", fmt.Errorf("invalid branch")
	}
	b, e := r.Git(ctx, "rev-parse", "--verify", "refs/heads/"+name+"^{commit}")
	return strings.TrimSpace(string(b)), e
}
func (r *Repository) Branches(ctx context.Context) ([]string, error) {
	b, e := r.Git(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}
func (r *Repository) Config(ctx context.Context, key string) string {
	b, _ := r.Git(ctx, "config", "--get", key)
	return strings.TrimSpace(string(b))
}

// ValidOID accepts full SHA-1 and SHA-256 object identifiers, never revisions.
func ValidOID(oid string) bool {
	if len(oid) != 40 && len(oid) != 64 {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}

type ReadStats struct{ GitCalls, TreeLoads, BlobBytes int64 }

func (r *Repository) ReadStats() ReadStats {
	return ReadStats{r.gitCalls.Load(), r.treeLoads.Load(), r.blobBytes.Load()}
}
func (r *Repository) Tree(ctx context.Context, oid string) (store.State, error) {
	if oid != "" && !ValidOID(oid) {
		return nil, fmt.Errorf("tree requires full object ID")
	}
	r.treeMu.Lock()
	defer r.treeMu.Unlock()
	if state, ok := r.trees[oid]; ok {
		return store.Clone(state), nil
	}
	r.treeLoads.Add(1)
	state := store.State{}
	if oid == "" {
		return state, nil
	}
	raw, err := r.Git(ctx, "ls-tree", "-rz", "--full-tree", oid)
	if err != nil {
		return nil, err
	}
	objects := []string{}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		header, path, ok := bytes.Cut(entry, []byte{'\t'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid git tree record")
		}
		if !r.managedPath(string(path)) && fields[0] != "160000" {
			objects = append(objects, fields[2])
		}
	}
	contents, err := r.readBlobs(ctx, objects)
	if err != nil {
		return nil, err
	}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		header, path, ok := bytes.Cut(entry, []byte{'\t'})
		if r.managedPath(string(path)) {
			continue
		}
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("invalid git tree record")
		}
		f := store.File{Mode: fields[0]}
		if f.Mode == "160000" {
			f.OID = fields[2]
		} else {
			f.Hash = contents[fields[2]]
		}
		state[string(path)] = f
	}
	// Bound state caching by entries and tree count; callers receive independent maps.
	if r.trees == nil {
		r.trees = map[string]store.State{}
	}
	if r.treeEntries+len(state) > 100000 || len(r.trees) >= 16 {
		r.trees = map[string]store.State{}
		r.treeEntries = 0
	}
	if len(state) <= 100000 {
		r.trees[oid] = store.Clone(state)
		r.treeEntries += len(state)
	}
	return state, nil
}
func validPath(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator)) && p != ".git" && !strings.HasPrefix(p, ".git/")
}
func (r *Repository) Worktree(ctx context.Context, base store.State, previous ...store.State) (store.State, error) {
	paths := map[string]bool{}
	gitlinks := map[string]string{}
	for p, f := range base {
		paths[p] = true
		if f.Mode == "160000" {
			gitlinks[p] = f.OID
		}
	}
	raw, err := r.Git(ctx, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		header, path, ok := bytes.Cut(entry, []byte{'\t'})
		h := strings.Fields(string(header))
		if !ok || len(h) != 3 {
			return nil, fmt.Errorf("invalid index entry")
		}
		if h[2] != "0" {
			return nil, fmt.Errorf("unmerged index is not supported")
		}
		p := string(path)
		paths[p] = true
		if h[0] == "160000" {
			gitlinks[p] = h[1]
		}
	}
	raw, err = r.Git(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) > 0 {
			paths[string(p)] = true
		}
	}
	scopeExit := map[string]store.File{}
	if len(previous) > 0 {
		for p, f := range previous[0] {
			if paths[p] {
				continue
			}
			if !validPath(p) {
				return nil, fmt.Errorf("unsafe previous path")
			}
			// check-ignore only evaluates rules; no contents outside the scope are read.
			_, e := r.Git(ctx, "check-ignore", "-q", "--", p)
			if e == nil {
				paths[p] = true
				f.Hash = ""
				f.OID = ""
				f.OutOfScope = true
				scopeExit[p] = f
			} else {
				var exit *exec.ExitError
				if !errors.As(e, &exit) || exit.ExitCode() != 1 {
					return nil, e
				}
			}
		}
	}
	state := store.State{}
	for p := range paths {
		if r.managedPath(p) {
			continue
		}
		if !validPath(p) {
			return nil, fmt.Errorf("unsafe repository path %q", p)
		}
		// Never follow a symlink in an ancestor directory outside the repository.
		blocked := false
		parent := filepath.Dir(p)
		for parent != "." {
			info, e := os.Lstat(filepath.Join(r.Root, parent))
			if errors.Is(e, os.ErrNotExist) {
				blocked = true
				break
			}
			if e != nil {
				return nil, e
			}
			if !info.IsDir() {
				blocked = true
				break
			}
			parent = filepath.Dir(parent)
		}
		if blocked {
			continue
		}
		full := filepath.Join(r.Root, p)
		info, e := os.Lstat(full)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if excluded, ok := scopeExit[p]; ok {
			state[p] = excluded
			continue
		}
		f := store.File{Mode: "100644"}
		var content []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, e := os.Readlink(full)
			if e != nil {
				return nil, e
			}
			f.Mode = "120000"
			content = []byte(target)
		case info.IsDir():
			if oid, ok := gitlinks[p]; ok {
				top, topErr := run(ctx, full, "rev-parse", "--show-toplevel")
				var b []byte
				var e error
				if topErr == nil && filepath.Clean(strings.TrimSpace(string(top))) == filepath.Clean(full) {
					b, e = run(ctx, full, "rev-parse", "--verify", "HEAD")
				} else {
					e = fmt.Errorf("submodule not initialized")
				}
				if e == nil {
					oid = strings.TrimSpace(string(b))
				}
				f.Mode = "160000"
				f.OID = oid
				state[p] = f
			}
			continue
		case info.Mode().IsRegular():
			content, e = os.ReadFile(full)
			if e != nil {
				return nil, e
			}
			if info.Mode().Perm()&0111 != 0 {
				f.Mode = "100755"
			}
		default:
			return nil, fmt.Errorf("unsupported special file %s", store.DisplayPath(p))
		}
		f.Hash = store.ContentHash(content)
		if original, ok := base[p]; !ok || original.Hash != f.Hash || original.OutOfScope {
			f.Hash, e = r.Store.PutBlob(content)
			if e != nil {
				return nil, e
			}
		}
		state[p] = f
	}
	return state, nil
}

type FileDiff struct {
	Path       string      `json:"path"`
	PathBase64 string      `json:"path_base64"`
	Old        *store.File `json:"old,omitempty"`
	New        *store.File `json:"new,omitempty"`
	Binary     bool        `json:"binary"`
	Patch      string      `json:"patch"`
}

func (r *Repository) diffUncached(ctx context.Context, before, after store.State) ([]FileDiff, error) {
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
	out := []FileDiff{}
	for _, p := range ordered {
		b, bok := before[p]
		a, aok := after[p]
		if bok == aok && a == b {
			continue
		}
		d := FileDiff{Path: store.DisplayPath(p), PathBase64: encodePath(p)}
		if bok {
			f := b
			d.Old = &f
		}
		if aok {
			f := a
			d.New = &f
		}
		if b.OutOfScope || a.OutOfScope {
			d.Patch = fmt.Sprintf("Capture scope changed for %s; out-of-scope contents are not captured.\n", d.Path)
			out = append(out, d)
			continue
		}
		var old, new []byte
		var err error
		if bok {
			old, err = r.content(b)
			if err != nil {
				return nil, err
			}
		}
		if aok {
			new, err = r.content(a)
			if err != nil {
				return nil, err
			}
		}
		d.Binary = bytes.Contains(old, []byte{0}) || bytes.Contains(new, []byte{0})
		header := fmt.Sprintf("diff --git a/%s b/%s\n", d.Path, d.Path)
		if !bok {
			header += "new file mode " + a.Mode + "\n"
		} else if !aok {
			header += "deleted file mode " + b.Mode + "\n"
		} else if b.Mode != a.Mode {
			header += "old mode " + b.Mode + "\nnew mode " + a.Mode + "\n"
		}
		if d.Binary {
			d.Patch = header + "Binary contents differ\n"
		} else {
			tmp, e := os.MkdirTemp("", "motif-diff-")
			if e != nil {
				return nil, e
			}
			oldpath, newpath := filepath.Join(tmp, "old"), filepath.Join(tmp, "new")
			e = os.WriteFile(oldpath, old, 0600)
			if e == nil {
				e = os.WriteFile(newpath, new, 0600)
			}
			if e != nil {
				os.RemoveAll(tmp)
				return nil, e
			}
			c := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--text", "--", oldpath, newpath)
			raw, e := c.CombinedOutput()
			os.RemoveAll(tmp)
			if e != nil {
				var exit *exec.ExitError
				if !errors.As(e, &exit) || exit.ExitCode() != 1 {
					return nil, fmt.Errorf("diff: %s: %w", raw, e)
				}
			}
			lines := strings.Split(string(raw), "\n")
			var body []string
			started := false
			for _, line := range lines {
				if strings.HasPrefix(line, "@@ ") {
					started = true
				}
				if started {
					body = append(body, line)
				}
			}
			if len(body) > 0 {
				oldlabel, newlabel := "a/"+d.Path, "b/"+d.Path
				if !bok {
					oldlabel = "/dev/null"
				}
				if !aok {
					newlabel = "/dev/null"
				}
				header += "--- " + oldlabel + "\n+++ " + newlabel + "\n" + strings.Join(body, "\n")
			}
			d.Patch = header
		}
		out = append(out, d)
	}
	return out, nil
}
func (r *Repository) content(f store.File) ([]byte, error) {
	if f.Mode == "160000" {
		return []byte("Subproject commit " + f.OID + "\n"), nil
	}
	return r.Store.Blob(f.Hash)
}

func (r *Repository) managedPath(path string) bool {
	candidate := filepath.Join(r.Root, path)
	for _, directory := range []string{r.GitDir, r.CommonDir} {
		relative, err := filepath.Rel(directory, candidate)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// readBlobs uses one Git process per tree instead of one process per path.
func (r *Repository) readBlobs(ctx context.Context, objects []string) (map[string]string, error) {
	result := map[string]string{}
	if len(objects) == 0 {
		return result, nil
	}
	var input strings.Builder
	seen := map[string]bool{}
	for _, oid := range objects {
		if !seen[oid] {
			input.WriteString(oid + "\n")
			seen[oid] = true
		}
	}
	cmd := exec.CommandContext(ctx, "git", "cat-file", "--batch")
	cmd.Dir = r.Root
	cmd.Stdin = strings.NewReader(input.String())
	r.gitCalls.Add(1)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	waited := false
	defer func() {
		if !waited {
			_ = pipe.Close()
			_ = cmd.Wait()
		}
	}()
	reader := bufio.NewReader(pipe)
	for range seen {
		header, e := reader.ReadString('\n')
		if e != nil {
			return nil, e
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("missing Git blob: %s", strings.TrimSpace(header))
		}
		size, e := strconv.ParseInt(fields[2], 10, 64)
		if e != nil || size < 0 {
			return nil, fmt.Errorf("invalid blob size")
		}
		data := make([]byte, size)
		if _, e = io.ReadFull(reader, data); e != nil {
			return nil, e
		}
		if term, e := reader.ReadByte(); e != nil || term != '\n' {
			return nil, fmt.Errorf("invalid cat-file framing")
		}
		object := fields[0]
		result[object] = r.Store.RegisterGitBlob(data, r.CommonDir+"/"+object, func() ([]byte, error) { return r.Git(context.Background(), "cat-file", "blob", object) })
		r.blobBytes.Add(size)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		return nil, err
	}
	return result, nil
}
