package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yhlooo/motif/internal/app/workspace"
)

const hookBegin = "# BEGIN MOTIF GIT HOOK\n"
const hookEnd = "# END MOTIF GIT HOOK\n"

type HooksPlan struct {
	Path      string
	Inherited string
	Override  bool
}

// PlanHooks leaves repository/worktree choices intact, but isolates inherited hooks.
// Git expands pathname values (including ~user), rather than Go guessing their meaning.
func PlanHooks(ctx context.Context, r *workspace.Project) (HooksPlan, error) {
	raw, err := r.Git(ctx, "config", "--show-scope", "--get", "core.hooksPath")
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return HooksPlan{}, err
		}
		return HooksPlan{Path: filepath.Join(r.CommonDir, "hooks")}, nil
	}
	scope, _, ok := strings.Cut(string(raw), "\t")
	if !ok {
		return HooksPlan{}, fmt.Errorf("cannot determine core.hooksPath scope")
	}
	raw, err = r.Git(ctx, "config", "--path", "--get", "core.hooksPath")
	if err != nil {
		return HooksPlan{}, err
	}
	value := strings.TrimSuffix(string(raw), "\n")
	if scope != "local" && scope != "worktree" {
		if scope == "command" {
			return HooksPlan{}, fmt.Errorf("command-scoped core.hooksPath cannot be overridden by repository config")
		}
		return HooksPlan{Path: filepath.Join(r.CommonDir, "hooks"), Inherited: value, Override: true}, nil
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(r.Root, value)
	}
	return HooksPlan{Path: filepath.Clean(value)}, nil
}

func readManifest(gitDir string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(gitDir, "motif", "install.json"))
	if os.IsNotExist(err) {
		return os.ReadFile(filepath.Join(gitDir, "motif-install.json"))
	}
	return raw, err
}
func ownerDirectory(r *workspace.Project, hooks string) string {
	if within(r.CommonDir, hooks) || within(r.GitDir, hooks) {
		return filepath.Join(r.CommonDir, "motif", "owners", fmt.Sprintf("%x", sha256.Sum256([]byte(hooks))))
	}
	return filepath.Join(hooks, ".motif-projects")
}
func backupPath(r *workspace.Project, hooks, name string) string {
	if within(r.CommonDir, hooks) || within(r.GitDir, hooks) {
		return filepath.Join(r.CommonDir, "motif", "original-hooks", fmt.Sprintf("%x", sha256.Sum256([]byte(hooks))), name)
	}
	return filepath.Join(hooks, ".motif", "original-hooks", name)
}
func ownedHook(r *workspace.Project, hooks, path string) (*File, error) {
	entries, err := os.ReadDir(ownerDirectory(r, hooks))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(ownerDirectory(r, hooks), entry.Name()))
		if err != nil {
			return nil, err
		}
		var owner struct {
			GitDir string `json:"git_dir"`
		}
		if err = json.Unmarshal(raw, &owner); err != nil {
			return nil, err
		}
		raw, err = readManifest(owner.GitDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var m Manifest
		if err = json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		for _, h := range m.Hooks {
			if h.Path == path {
				return &h, nil
			}
		}
	}
	return nil, nil
}
func managedLocalHooksPath(r *workspace.Project) (string, error) {
	roots, err := r.Git(context.Background(), "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", err
	}
	for _, part := range bytes.Split(roots, []byte{0}) {
		if !bytes.HasPrefix(part, []byte("worktree ")) {
			continue
		}
		root := string(bytes.TrimPrefix(part, []byte("worktree ")))
		other, err := workspace.Open(context.Background(), root, r.Store)
		if err != nil {
			return "", err
		}
		m, err := Load(other)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if m.LocalHooksPath != "" {
			return m.LocalHooksPath, nil
		}
	}
	return "", nil
}

func verifyHook(h File, allowMissing bool) error {
	if h.Backup != "" {
		info, err := os.Lstat(h.Backup)
		if err != nil {
			return fmt.Errorf("original hook backup is unavailable: %w", err)
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("original hook backup was changed: %s", h.Backup)
		}
	}
	data, err := os.ReadFile(h.Path)
	if allowMissing && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if string(data) != h.Content {
		return fmt.Errorf("Motif hook was changed: %s", h.Path)
	}
	info, err := os.Stat(h.Path)
	if err != nil {
		return err
	}
	if !allowMissing && info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("Motif hook is not executable: %s", h.Path)
	}
	return nil
}

// File transactions retain symlinks and original modes as well as bytes.
type savedFile struct {
	path   string
	data   []byte
	mode   os.FileMode
	link   string
	exists bool
}
type fileTransaction struct {
	files              []savedFile
	committed, changed bool
}

func (t *fileTransaction) remember(path string) error {
	for _, f := range t.files {
		if f.path == path {
			return nil
		}
	}
	f := savedFile{path: path}
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		f.exists, f.mode = true, info.Mode()
		if info.Mode()&os.ModeSymlink != 0 {
			f.link, err = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			f.data, err = os.ReadFile(path)
		} else {
			return fmt.Errorf("refuse to replace non-file %s", path)
		}
		if err != nil {
			return err
		}
	}
	t.files = append(t.files, f)
	return nil
}
func (t *fileTransaction) write(path string, data []byte, mode os.FileMode) error {
	if err := t.remember(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err == nil {
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Keep existing configuration permissions; hooks must be executable.
		if mode == 0600 {
			mode = info.Mode().Perm()
		}
		if bytes.Equal(old, data) && info.Mode().Perm() == mode {
			return nil
		}
	}
	t.changed = true
	return atomic(path, data, mode)
}
func (t *fileTransaction) remove(path string) error {
	if err := t.remember(path); err != nil {
		return err
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err == nil {
		t.changed = true
	}
	return err
}
func (t *fileTransaction) move(from, to string) error {
	if err := t.remember(from); err != nil {
		return err
	}
	if err := t.remember(to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return err
	}
	t.changed = true
	return os.Rename(from, to)
}
func (t *fileTransaction) rollback() {
	if t.committed {
		return
	}
	for i := len(t.files) - 1; i >= 0; i-- {
		f := t.files[i]
		if !f.exists {
			_ = os.Remove(f.path)
			continue
		}
		if f.link != "" {
			_ = os.Remove(f.path)
			_ = os.Symlink(f.link, f.path)
		} else {
			_ = atomic(f.path, f.data, f.mode.Perm())
		}
	}
}
