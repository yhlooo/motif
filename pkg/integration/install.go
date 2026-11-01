// Package integration manages only project-local, explicitly marked hooks.
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
	OwnerFile     string `json:"owner_file,omitempty"`
	ConfigCreated bool   `json:"config_created,omitempty"`
	Config        File   `json:"config"`
	Hooks         []File `json:"hooks"`
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
		complete := map[string]bool{}
		for _, hook := range m.Hooks {
			complete[filepath.Base(hook.Path)] = true
		}
		if complete["post-commit"] && complete["post-checkout"] && complete["post-rewrite"] && complete["reference-transaction"] {
			return m, false, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return m, false, err
	}
	// Shared hooksPath must not cause project installation to edit another project or global hooks.
	hooks := r.Config(ctx, "core.hooksPath")
	if hooks == "" {
		hooks = filepath.Join(r.CommonDir, "hooks")
	} else if !filepath.IsAbs(hooks) {
		hooks = filepath.Join(r.Root, hooks)
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
	if existing != nil && string(existing) != m.Config.Content {
		return m, false, fmt.Errorf("unmanaged Motif block already exists; inspect %s", config)
	}
	next := append([]byte{}, raw...)
	if existing == nil {
		_, statErr := os.Stat(config)
		m.ConfigCreated = os.IsNotExist(statErr)
		m.Config = File{Path: config, Content: configBlock(binary)}
		next = append(next, []byte("\n"+m.Config.Content)...)
	}
	m.Hooks = nil
	if err = validateTOML(next); err != nil {
		return m, false, fmt.Errorf("cannot merge Codex hooks: %w", err)
	}
	for _, name := range []string{"post-commit", "post-checkout", "post-rewrite", "reference-transaction"} {
		path := filepath.Join(hooks, name)
		_, err := os.Lstat(path)
		backup := ""
		shared := false
		if err == nil {
			info, e := os.Lstat(path)
			if e != nil {
				return m, false, e
			}
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				return m, false, fmt.Errorf("refuse to replace non-file hook %s", path)
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return m, false, e
			}
			shared = bytes.HasPrefix(data, []byte("#!/bin/sh\n# Managed by Motif;"))
			backup = path + ".motif-original"
			if shared {
				if _, e = os.Lstat(backup); os.IsNotExist(e) {
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
		content := hookScript(binary, name, backup)
		if !within(r.Root, hooks) && !within(r.CommonDir, hooks) {
			content = strings.Replace(content, " hook git ", " hook git --installed-only ", 1)
		}

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
	m.OwnerFile = filepath.Join(hooks, ".motif-projects", fmt.Sprintf("%x.json", sha256.Sum256([]byte(r.GitDir))))
	ownerData, e := json.Marshal(map[string]string{"git_dir": r.GitDir})
	if e != nil {
		rollback()
		return m, false, e
	}
	if err = write(m.OwnerFile, ownerData, 0600); err != nil {
		rollback()
		return m, false, err
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
		if h.Backup != "" {
			info, e := os.Lstat(h.Backup)
			if e != nil {
				return fmt.Errorf("original hook backup is unavailable: %w", e)
			}
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("original hook backup was changed: %s", h.Backup)
			}
		}
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
	if m.ConfigCreated && strings.TrimSpace(string(next)) == "" {
		if err = os.Remove(m.Config.Path); err != nil {
			return err
		}
		_ = os.Remove(filepath.Dir(m.Config.Path))
	} else if err = atomic(m.Config.Path, next, info.Mode().Perm()); err != nil {
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
	if m.OwnerFile != "" {
		if err = os.Remove(m.OwnerFile); err != nil && !os.IsNotExist(err) {
			return err
		}
		_ = os.Remove(filepath.Dir(m.OwnerFile))
	}
	return os.Remove(manifestPath(r))
}

func within(parent, path string) bool {
	rel, e := filepath.Rel(parent, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func otherOwners(r *gitrepo.Repository, m Manifest) (map[string]bool, error) {
	keep := map[string]bool{}
	if len(m.Hooks) > 0 {
		directory := filepath.Join(filepath.Dir(m.Hooks[0].Path), ".motif-projects")
		entries, e := os.ReadDir(directory)
		if e != nil && !os.IsNotExist(e) {
			return nil, e
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			data, e := os.ReadFile(filepath.Join(directory, entry.Name()))
			if e != nil {
				return nil, e
			}
			var owner struct {
				GitDir string `json:"git_dir"`
			}
			if e = json.Unmarshal(data, &owner); e != nil {
				return nil, e
			}
			if owner.GitDir == r.GitDir {
				continue
			}
			data, e = os.ReadFile(filepath.Join(owner.GitDir, "motif-install.json"))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			var other Manifest
			if e = json.Unmarshal(data, &other); e != nil {
				return nil, e
			}
			for _, h := range other.Hooks {
				for _, ours := range m.Hooks {
					if h.Path == ours.Path {
						keep[h.Path] = true
					}
				}
			}
		}
	}
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

func hookScript(binary, name, backup string) string {
	content := "#!/bin/sh\n# Managed by Motif; see motif-install.json\nstatus=0\n"
	input := ""
	if name == "post-rewrite" || name == "reference-transaction" {
		content += "input=$(mktemp) || {\n"
		if backup != "" {
			content += "  if [ -x " + quote(backup) + " ]; then " + quote(backup) + " \"$@\"; exit $?; fi\n"
		}
		content += "  exit 0\n}\ntrap 'rm -f \"$input\"' EXIT HUP INT TERM\ncat >\"$input\" || exit 0\n"
		input = " <\"$input\""
	}
	if backup != "" {
		content += "if [ -x " + quote(backup) + " ]; then\n  " + quote(backup) + " \"$@\"" + input + " || status=$?\nfi\n"
	}
	content += quote(binary) + " hook git " + name + " \"$@\"" + input + " || true\nexit \"$status\"\n"
	return content
}

// HasGitHook limits a shared hooksPath wrapper to projects that installed it.
func HasGitHook(ctx context.Context, dir, event string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--absolute-git-dir")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(raw)), "motif-install.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var m Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return false, err
	}
	for _, hook := range m.Hooks {
		if filepath.Base(hook.Path) == event {
			return true, nil
		}
	}
	return false, nil
}
