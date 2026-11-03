package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yhlooo/motif/internal/app/workspace"
)

// RemovalPlan selects configurations, and keeps Git hooks while any agent remains.
func RemovalPlan(m Manifest, agents []string) Manifest {
	remove := m
	if len(agents) == 0 {
		return remove
	}
	remove.Config = File{}
	remove.ConfigCreated = false
	remove.Claude = nil
	for _, agent := range agents {
		if agent == "codex" {
			remove.Config, remove.ConfigCreated = m.Config, m.ConfigCreated
		}
		if agent == "claude" {
			remove.Claude = m.Claude
		}
	}
	if (m.Config.Path != "" && remove.Config.Path == "") || (m.Claude != nil && remove.Claude == nil) {
		remove.Hooks = nil
		remove.OwnerFile = ""
		remove.LocalHooksPath = ""
	}
	return remove
}

func Uninstall(r *workspace.Project, m Manifest) error { return UninstallAgents(r, m, nil) }

func UninstallAgents(r *workspace.Project, m Manifest, agents []string) error {
	removal := RemovalPlan(m, agents)
	if err := verify(removal, true); err != nil {
		return fmt.Errorf("preserving modified configuration: %w", err)
	}
	remaining := m
	tx := &fileTransaction{}
	defer tx.rollback()
	if removal.Config.Path != "" {
		data, err := os.ReadFile(removal.Config.Path)
		if err != nil {
			return err
		}
		next := bytes.Replace(data, []byte(removal.Config.Content), nil, 1)
		if err = validateTOML(next); err != nil {
			return err
		}
		if removal.ConfigCreated && strings.TrimSpace(string(next)) == "" {
			if err = tx.remove(removal.Config.Path); err != nil {
				return err
			}
		} else if err = tx.write(removal.Config.Path, next, 0600); err != nil {
			return err
		}
		remaining.Config, remaining.ConfigCreated = File{}, false
	}
	if removal.Claude != nil {
		next, err := removeClaude(removal.Claude, false)
		if err != nil {
			return err
		}
		if next == nil {
			err = tx.remove(removal.Claude.Path)
		} else {
			err = tx.write(removal.Claude.Path, next, 0600)
		}
		if err != nil {
			return err
		}
		remaining.Claude = nil
	}
	if remaining.Config.Path != "" || remaining.Claude != nil {
		data, err := json.MarshalIndent(remaining, "", "  ")
		if err != nil {
			return err
		}
		if err = tx.write(manifestPath(r), data, 0600); err != nil {
			return err
		}
	} else {
		keep, err := otherOwners(r, m)
		if err != nil {
			return err
		}
		for _, h := range m.Hooks {
			if keep[h.Path] {
				continue
			}
			if err = tx.remove(h.Path); err != nil {
				return err
			}
			if h.Backup != "" {
				if err = tx.move(h.Backup, h.Path); err != nil {
					return err
				}
			}
		}
		if m.OwnerFile != "" {
			if err = tx.remove(m.OwnerFile); err != nil {
				return err
			}
		}
		if m.LocalHooksPath != "" && len(keep) == 0 {
			current, e := r.Git(context.Background(), "config", "--local", "--get", "core.hooksPath")
			if e == nil && strings.TrimSpace(string(current)) == m.LocalHooksPath {
				if err = tx.remember(filepath.Join(r.CommonDir, "config")); err != nil {
					return err
				}
				if _, err = r.Git(context.Background(), "config", "--local", "--unset-all", "core.hooksPath"); err != nil {
					return err
				}
			}
		}
		for _, path := range []string{manifestPath(r), filepath.Join(r.GitDir, "motif-install.json")} {
			if err = tx.remove(path); err != nil {
				return err
			}
		}
	}
	tx.committed = true
	return nil
}
