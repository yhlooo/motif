// Package integration manages only project-local, explicitly marked hooks.
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/yhlooo/motif/internal/i18n"
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
	OwnerFile      string `json:"owner_file,omitempty"`
	ConfigCreated  bool   `json:"config_created,omitempty"`
	Config         File   `json:"config"`
	Hooks          []File `json:"hooks"`
	LocalHooksPath string `json:"local_hooks_path,omitempty"`
}

func manifestPath(r *workspace.Project) string {
	return filepath.Join(r.GitDir, "motif", "install.json")
}
func Load(r *workspace.Project) (Manifest, error) {
	var m Manifest
	raw, e := readManifest(r.GitDir)
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
func configBlock(ctx context.Context, binary string) string {
	var b strings.Builder
	b.WriteString(begin)
	for _, hook := range codexHooks {
		fmt.Fprintf(&b, "[[hooks.%s]]\n[[hooks.%s.hooks]]\ntype = \"command\"\ncommand = %s\nstatusMessage = %s\ntimeout = 30\nasync = false\n", hook.event, hook.event, strconv.Quote(quote(binary)+" hook codex || true"), strconv.Quote(i18n.TContext(ctx, hook.statusMessage)))
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

// Install repairs missing integrations as well as installing new ones.
func Install(ctx context.Context, r *workspace.Project, binary string) (Manifest, bool, error) {
	return InstallWithOptions(ctx, r, binary, InstallOptions{})
}

type InstallOptions struct{ ConfirmLocalHooksPath bool }

func InstallWithOptions(ctx context.Context, r *workspace.Project, binary string, options InstallOptions) (Manifest, bool, error) {
	old, err := Load(r)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return old, false, err
	}
	plan, err := PlanHooks(ctx, r)
	if err != nil {
		return old, false, err
	}
	if plan.Override && !options.ConfirmLocalHooksPath {
		return old, false, fmt.Errorf("switching inherited core.hooksPath to %s requires confirmation; global hooks will no longer run in this repository", plan.Path)
	}
	m := old
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
	if existing != nil && string(existing) != old.Config.Content {
		return m, false, fmt.Errorf("Motif config was changed; inspect %s", config)
	}
	next := append([]byte{}, raw...)
	if existing == nil {
		m.ConfigCreated = false
		if _, e := os.Stat(config); os.IsNotExist(e) {
			m.ConfigCreated = true
		}
		m.Config = File{Path: config, Content: configBlock(ctx, binary)}
		next = append(next, []byte("\n"+m.Config.Content)...)
	} else {
		// Only update a block that still matches our recorded installation.
		m.Config = File{Path: config, Content: configBlock(ctx, binary)}
		next = bytes.Replace(next, existing, []byte(m.Config.Content), 1)
	}
	if err = validateTOML(next); err != nil {
		return m, false, err
	}
	m.Hooks = nil
	backupMoves := map[string]string{}
	// Validate all destinations and former hooks before making any changes.
	for _, name := range []string{"post-commit", "post-checkout", "post-rewrite", "reference-transaction"} {
		path := filepath.Join(plan.Path, name)
		h := File{Path: path}
		var known *File
		fromOther := false
		for _, previous := range old.Hooks {
			if previous.Path == path {
				copy := previous
				known = &copy
			}
		}
		if known == nil {
			known, err = ownedHook(r, plan.Path, path)
			fromOther = known != nil
			if err != nil {
				return m, false, err
			}
		}
		data, e := os.ReadFile(path)
		if e != nil && !os.IsNotExist(e) {
			return m, false, e
		}
		if info, statErr := os.Lstat(path); statErr == nil {
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				return m, false, fmt.Errorf("refuse to replace non-file hook %s", path)
			}
			if info.Mode()&os.ModeSymlink != 0 && os.IsNotExist(e) {
				return m, false, fmt.Errorf("refuse to replace dangling hook symlink %s", path)
			}
		} else if !os.IsNotExist(statErr) {
			return m, false, statErr
		}
		if known != nil && (os.IsNotExist(e) || string(data) == known.Content) {
			h.Backup = known.Backup
			h.Shared = known.Shared || fromOther
		} else if e == nil {
			if bytes.Contains(data, []byte(hookBegin)) || bytes.Contains(data, []byte(hookEnd)) || bytes.HasPrefix(data, []byte("#!/bin/sh\n# Managed by Motif;")) {
				return m, false, fmt.Errorf("modified or unregistered Motif hook: %s", path)
			}
			h.Backup = backupPath(r, plan.Path, name)
			if _, e = os.Lstat(h.Backup); !os.IsNotExist(e) {
				return m, false, fmt.Errorf("backup already exists or cannot be inspected: %s", h.Backup)
			}
		}
		if h.Backup != "" && known != nil && h.Backup == known.Backup {
			if _, e = os.Lstat(h.Backup); e != nil {
				return m, false, fmt.Errorf("original hook backup is unavailable: %w", e)
			}
		}
		// Move legacy backups out of .git/hooks into the Motif metadata directory.
		if known != nil && h.Backup != "" && within(r.CommonDir, h.Backup) && !within(filepath.Join(r.CommonDir, "motif"), h.Backup) && !fromOther {
			destination := backupPath(r, plan.Path, name)
			if _, e := os.Lstat(destination); !os.IsNotExist(e) {
				return m, false, fmt.Errorf("backup destination already exists: %s", destination)
			}
			backupMoves[h.Backup] = destination
			h.Backup = destination
		}
		h.Content = hookScript(binary, name, h.Backup)
		if !within(r.Root, plan.Path) && !within(r.CommonDir, plan.Path) {
			h.Content = strings.Replace(h.Content, " hook git ", " hook git --installed-only ", 1)
		}
		if known != nil && known.Content != h.Content && fromOther {
			return m, false, fmt.Errorf("shared Motif hook conflicts with this binary: %s", path)
		}
		m.Hooks = append(m.Hooks, h)
	}
	keep, err := otherOwners(r, old)
	if err != nil {
		return m, false, err
	}
	for _, h := range old.Hooks {
		if keep[h.Path] {
			for _, updated := range m.Hooks {
				if updated.Path == h.Path && updated.Content != h.Content {
					return m, false, fmt.Errorf("shared hook must remain compatible with its other owners: %s", h.Path)
				}
			}
		}
		if filepath.Dir(h.Path) == plan.Path || keep[h.Path] {
			continue
		}
		if err = verifyHook(h, true); err != nil {
			return m, false, err
		}
	}
	tx := &fileTransaction{}
	defer tx.rollback()
	if err = tx.write(config, next, 0600); err != nil {
		return m, false, err
	}
	for source, destination := range backupMoves {
		if err = tx.move(source, destination); err != nil {
			return m, false, err
		}
	}
	for _, h := range m.Hooks {
		if h.Backup != "" {
			if _, e := os.Lstat(h.Backup); os.IsNotExist(e) {
				if err = tx.move(h.Path, h.Backup); err != nil {
					return m, false, err
				}
			}
		}
		if err = tx.write(h.Path, []byte(h.Content), 0755); err != nil {
			return m, false, err
		}
	}
	for _, h := range old.Hooks {
		if filepath.Dir(h.Path) == plan.Path || keep[h.Path] {
			continue
		}
		if err = tx.remove(h.Path); err != nil {
			return m, false, err
		}
		if h.Backup != "" {
			if err = tx.move(h.Backup, h.Path); err != nil {
				return m, false, err
			}
		}
	}
	m.OwnerFile = filepath.Join(ownerDirectory(r, plan.Path), fmt.Sprintf("%x.json", sha256.Sum256([]byte(r.GitDir))))
	ownerData, _ := json.Marshal(map[string]string{"git_dir": r.GitDir})
	if err = tx.write(m.OwnerFile, ownerData, 0600); err != nil {
		return m, false, err
	}
	if old.OwnerFile != "" && old.OwnerFile != m.OwnerFile {
		if err = tx.remove(old.OwnerFile); err != nil {
			return m, false, err
		}
	}
	if plan.Override {
		m.LocalHooksPath = plan.Path
	}
	// Carry ownership of a repository-level override across linked worktrees.
	if m.LocalHooksPath == "" {
		m.LocalHooksPath, err = managedLocalHooksPath(r)
		if err != nil {
			return m, false, err
		}
	}
	if plan.Override {
		configPath := filepath.Join(r.CommonDir, "config")
		if err = tx.remember(configPath); err != nil {
			return m, false, err
		}
		if _, err = r.Git(ctx, "config", "--local", "core.hooksPath", plan.Path); err != nil {
			return m, false, err
		}
		tx.changed = true
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, false, err
	}
	if err = tx.write(manifestPath(r), data, 0600); err != nil {
		return m, false, err
	}
	if err = tx.remove(filepath.Join(r.GitDir, "motif-install.json")); err != nil {
		return m, false, err
	}
	changed := tx.changed
	tx.committed = true
	return m, changed, nil
}
func Verify(m Manifest) error { return verify(m, false) }
func verify(m Manifest, allowMissing bool) error {
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
		if err := verifyHook(h, allowMissing); err != nil {
			return err
		}
	}

	return nil
}
func Uninstall(r *workspace.Project, m Manifest) error {
	if err := verify(m, true); err != nil {
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
		if err = os.Remove(h.Path); err != nil && !os.IsNotExist(err) {
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
	if m.LocalHooksPath != "" && len(keep) == 0 {
		current, e := r.Git(context.Background(), "config", "--local", "--get", "core.hooksPath")
		if e == nil && strings.TrimSpace(string(current)) == m.LocalHooksPath {
			if _, e = r.Git(context.Background(), "config", "--local", "--unset-all", "core.hooksPath"); e != nil {
				return e
			}
		}
	}
	for _, path := range []string{manifestPath(r), filepath.Join(r.GitDir, "motif-install.json")} {
		if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func within(parent, path string) bool {
	rel, e := filepath.Rel(parent, path)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func otherOwners(r *workspace.Project, m Manifest) (map[string]bool, error) {
	keep := map[string]bool{}
	if len(m.Hooks) > 0 {
		directory := ownerDirectory(r, filepath.Dir(m.Hooks[0].Path))
		if m.OwnerFile != "" {
			directory = filepath.Dir(m.OwnerFile)
		}
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
			data, e = readManifest(owner.GitDir)
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
		other, e := workspace.Open(context.Background(), root, r.Store)
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
	content := "#!/bin/sh\n" + hookBegin + "# https://github.com/yhlooo/motif\nstatus=0\n"
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
	return content + hookEnd
}

// HasGitHook limits a shared hooksPath wrapper to projects that installed it.
func HasGitHook(ctx context.Context, dir, event string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--absolute-git-dir")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		return false, err
	}
	data, err := readManifest(strings.TrimSpace(string(raw)))
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
