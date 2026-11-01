// Package integration manages only project-local, explicitly marked hooks.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/yhlooo/motif/pkg/gitrepo"
)

const begin = "# BEGIN MOTIF CODEX\n"
const end = "# END MOTIF CODEX\n"

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Backup  string `json:"backup,omitempty"`
	Shared  bool   `json:"shared,omitempty"`
}
type Manifest struct {
	Config File   `json:"config"`
	Hooks  []File `json:"hooks"`
}

func manifestPath(r *gitrepo.Repository) string { return filepath.Join(r.GitDir, "motif-install.json") }
func Load(r *gitrepo.Repository) (Manifest, error) {
	var m Manifest
	raw, e := os.ReadFile(manifestPath(r))
	if e != nil {
		return m, e
	}
	e = json.Unmarshal(raw, &m)
	return m, e
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func atomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".motif-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func configBlock(binary string) string {
	var b strings.Builder
	b.WriteString(begin)
	for _, event := range []string{"PreToolUse", "PostToolUse", "Stop", "Interrupt", "SessionEnd"} {
		fmt.Fprintf(&b, "[[hooks.%s]]\n[[hooks.%s.hooks]]\ntype = \"command\"\ncommand = %s\ntimeout = 30\nasync = false\n", event, event, strconv.Quote(quote(binary)+" hook codex || true"))
	}
	b.WriteString(end)
	return b.String()
}
func block(data []byte) ([]byte, error) {
	a := bytes.Index(data, []byte(begin))
	b := bytes.Index(data, []byte(end))
	if a < 0 && b < 0 {
		return nil, nil
	}
	if a < 0 || b < a || bytes.Count(data, []byte(begin)) != 1 || bytes.Count(data, []byte(end)) != 1 {
		return nil, fmt.Errorf("conflicting Motif config markers")
	}
	return data[a : b+len(end)], nil
}
func validateTOML(data []byte) error { var v map[string]any; return toml.Unmarshal(data, &v) }
func Install(ctx context.Context, r *gitrepo.Repository, binary string) (Manifest, bool, error) {
	m, err := Load(r)
	if err == nil {
		if err = Verify(m); err != nil {
			return m, false, err
		}
		return m, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return m, false, err
	}
	// Shared hooksPath must not cause project installation to edit another project or global hooks.
	hooks := r.Config(ctx, "core.hooksPath")
	if hooks == "" {
		hooks = filepath.Join(r.CommonDir, "hooks")
	} else if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(r.Root, hooks)
	}
	if !within(r.Root, hooks) && !within(r.CommonDir, hooks) {
		return m, false, fmt.Errorf("core.hooksPath is outside this project; shared global hook directories need manual integration: %s", hooks)
	}
	if resolved, e := filepath.EvalSymlinks(hooks); e == nil && !within(r.Root, resolved) && !within(r.CommonDir, resolved) {
		return m, false, fmt.Errorf("hook directory resolves outside this project")
	}
	config := filepath.Join(r.Root, ".codex", "config.toml")
	if info, e := os.Lstat(config); e == nil && !info.Mode().IsRegular() {
		return m, false, fmt.Errorf("refuse to replace non-regular Codex config")
	}

	raw, err := os.ReadFile(config)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return m, false, err
	}
	if err = validateTOML(raw); err != nil {
		return m, false, err
	}
	existing, err := block(raw)
	if err != nil {
		return m, false, err
	}
	if existing != nil {
		return m, false, fmt.Errorf("unmanaged Motif block already exists; inspect %s", config)
	}
	m.Config = File{Path: config, Content: configBlock(binary)}
	next := append(append([]byte{}, raw...), []byte("\n"+m.Config.Content)...)
	if err = validateTOML(next); err != nil {
		return m, false, fmt.Errorf("cannot merge Codex hooks: %w", err)
	}
	for _, name := range []string{"post-commit", "post-checkout"} {
		path := filepath.Join(hooks, name)
		_, err := os.Lstat(path)
		backup := ""
		shared := false
		if err == nil {
			info, e := os.Lstat(path)
			if e != nil {
				return m, false, e
			}
			if !info.Mode().IsRegular() {
				return m, false, fmt.Errorf("refuse to replace non-regular hook %s", path)
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return m, false, e
			}
			shared = bytes.HasPrefix(data, []byte("#!/bin/sh\n# Managed by Motif;"))
			backup = path + ".motif-original"
			if shared {
				if _, e = os.Stat(backup); os.IsNotExist(e) {
					backup = ""
				} else if e != nil {
					return m, false, e
				}
			}
			if !shared {
				if _, e = os.Lstat(backup); e == nil {
					return m, false, fmt.Errorf("backup already exists: %s", backup)
				} else if !os.IsNotExist(e) {
					return m, false, e
				}
			}
		} else if !os.IsNotExist(err) {
			return m, false, err
		}
		content := "#!/bin/sh\n# Managed by Motif; see motif-install.json\nstatus=0\n"
		if backup != "" {
			content += "if [ -x " + quote(backup) + " ]; then\n  " + quote(backup) + " \"$@\" || status=$?\nfi\n"
		}
		content += quote(binary) + " hook git " + name + " \"$@\" || true\nexit \"$status\"\n"
		if shared {
			data, e := os.ReadFile(path)
			if e != nil {
				return m, false, e
			}
			if string(data) != content {
				return m, false, fmt.Errorf("shared Motif hook conflicts with this binary: %s", path)
			}
		}
		m.Hooks = append(m.Hooks, File{Path: path, Content: content, Backup: backup, Shared: shared})
	}
	// Stage validation before touching files; roll back ordinary write failures.
	type original struct {
		path   string
		data   []byte
		mode   os.FileMode
		exists bool
	}
	var originals []original
	write := func(path string, data []byte, mode os.FileMode) error {
		old, e := os.ReadFile(path)
		exists := e == nil
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		oldMode := mode
		if info, e := os.Stat(path); e == nil {
			oldMode = info.Mode().Perm()
		}
		originals = append(originals, original{path, old, oldMode, exists})
		return atomic(path, data, mode)
	}
	rollback := func() {
		for i := len(originals) - 1; i >= 0; i-- {
			o := originals[i]
			if o.exists {
				_ = atomic(o.path, o.data, o.mode)
			} else {
				_ = os.Remove(o.path)
			}
		}
		for _, h := range m.Hooks {
			if h.Backup != "" && !h.Shared {
				if _, e := os.Stat(h.Backup); e == nil {
					_ = os.Remove(h.Path)
					_ = os.Rename(h.Backup, h.Path)
				}
			}
		}
	}
	mode := os.FileMode(0600)
	if info, e := os.Stat(config); e == nil {
		mode = info.Mode().Perm()
	}
	if err = write(config, next, mode); err != nil {
		rollback()
		return m, false, err
	}
	for _, h := range m.Hooks {
		if h.Shared {
			continue
		}
		if h.Backup != "" {
			if err = os.Rename(h.Path, h.Backup); err != nil {
				rollback()
				return m, false, err
			}
		}
		if err = write(h.Path, []byte(h.Content), 0755); err != nil {
			rollback()
			return m, false, err
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = write(manifestPath(r), data, 0600)
	}
	if err != nil {
		rollback()
		return m, false, err
	}
	return m, true, nil
}
func Verify(m Manifest) error {
	data, err := os.ReadFile(m.Config.Path)
	if err != nil {
		return err
	}
	if err = validateTOML(data); err != nil {
		return err
	}
	b, err := block(data)
	if err != nil {
		return err
	}
	if string(b) != m.Config.Content {
		return fmt.Errorf("Motif config was changed: %s", m.Config.Path)
	}
	for _, h := range m.Hooks {
		data, e := os.ReadFile(h.Path)
		if e != nil {
			return e
		}
		if string(data) != h.Content {
			return fmt.Errorf("Motif hook was changed: %s", h.Path)
		}
		info, e := os.Stat(h.Path)
		if e != nil {
			return e
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("Motif hook is not executable: %s", h.Path)
		}
	}
	return nil
}
func Uninstall(r *gitrepo.Repository, m Manifest) error {
	if err := Verify(m); err != nil {
		return fmt.Errorf("preserving modified configuration: %w", err)
	}
	keep, err := otherOwners(r, m)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(m.Config.Path)
	if err != nil {
		return err
	}
	next := bytes.Replace(data, []byte(m.Config.Content), nil, 1)
	if err = validateTOML(next); err != nil {
		return err
	}
	info, err := os.Stat(m.Config.Path)
	if err != nil {
		return err
	}
	if err = atomic(m.Config.Path, next, info.Mode().Perm()); err != nil {
		return err
	}
	for _, h := range m.Hooks {
		if keep[h.Path] {
			continue
		}
		if err = os.Remove(h.Path); err != nil {
			return err
		}
		if h.Backup != "" {
			if err = os.Rename(h.Backup, h.Path); err != nil {
				return err
			}
		}
	}
	return os.Remove(manifestPath(r))
}

func within(parent, path string) bool {
	rel, e := filepath.Rel(parent, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func otherOwners(r *gitrepo.Repository, m Manifest) (map[string]bool, error) {
	keep := map[string]bool{}
	raw, err := r.Git(context.Background(), "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	for _, part := range bytes.Split(raw, []byte{0}) {
		if !bytes.HasPrefix(part, []byte("worktree ")) {
			continue
		}
		root := string(bytes.TrimPrefix(part, []byte("worktree ")))
		if root == r.Root {
			continue
		}
		other, e := gitrepo.Open(context.Background(), root, r.Store)
		if e != nil {
			return nil, e
		}
		owner, e := Load(other)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, h := range owner.Hooks {
			for _, ours := range m.Hooks {
				if h.Path == ours.Path {
					keep[h.Path] = true
				}
			}
		}
	}
	return keep, nil
}
