package integration

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

// FileChange describes an installation's effect on a file, before any writes.
type FileChange struct{ Path, Action string }

type installPlan struct {
	before     fileTransaction
	state      map[string]savedFile
	operations []func(*fileTransaction) error
}

func newInstallPlan() *installPlan { return &installPlan{state: map[string]savedFile{}} }

func (p *installPlan) file(path string) (savedFile, error) {
	if f, ok := p.state[path]; ok {
		return f, nil
	}
	if err := p.before.remember(path); err != nil {
		return savedFile{}, err
	}
	f := p.before.files[len(p.before.files)-1]
	// Remember target bytes too, so a changed symlink target invalidates preview.
	if f.link != "" {
		info, err := os.Stat(path)
		if err != nil {
			return f, err
		}
		f.data, err = os.ReadFile(path)
		if err != nil {
			return f, err
		}
		f.mode = info.Mode()
		p.before.files[len(p.before.files)-1].data = f.data
	}
	p.state[path] = f
	return f, nil
}

func (p *installPlan) write(path string, data []byte, mode os.FileMode) error {
	f, err := p.file(path)
	if err != nil {
		return err
	}
	if f.exists && mode == 0600 {
		mode = f.mode.Perm()
	}
	if f.exists && bytes.Equal(f.data, data) && f.mode.Perm() == mode {
		return nil
	}
	p.operations = append(p.operations, func(t *fileTransaction) error { return t.write(path, data, mode) })
	p.state[path] = savedFile{path: path, data: data, mode: mode, exists: true}
	return nil
}

func (p *installPlan) remove(path string) error {
	f, err := p.file(path)
	if err != nil {
		return err
	}
	if !f.exists {
		return nil
	}
	p.operations = append(p.operations, func(t *fileTransaction) error { return t.remove(path) })
	p.state[path] = savedFile{path: path}
	return nil
}

func (p *installPlan) move(from, to string) error {
	f, err := p.file(from)
	if err != nil {
		return err
	}
	if !f.exists {
		return fmt.Errorf("move source is missing: %s", from)
	}
	if _, err = p.file(to); err != nil {
		return err
	}
	p.operations = append(p.operations, func(t *fileTransaction) error { return t.move(from, to) })
	f.path = to
	p.state[to] = f
	p.state[from] = savedFile{path: from}
	return nil
}

func (p *installPlan) configure(path string, configure func() error) error {
	f, err := p.file(path)
	if err != nil {
		return err
	}
	// The exact Git config encoding belongs to Git; mark its bytes as changed.
	f.data = nil
	p.state[path] = f
	p.operations = append(p.operations, func(t *fileTransaction) error {
		if err := t.remember(path); err != nil {
			return err
		}
		t.changed = true
		return configure()
	})
	return nil
}

func (p *installPlan) changes() []FileChange {
	var changes []FileChange
	for _, before := range p.before.files {
		after := p.state[before.path]
		modeEqual := before.mode.Perm() == after.mode.Perm() || before.link != "" && before.link == after.link
		if before.exists == after.exists && before.link == after.link && modeEqual && bytes.Equal(before.data, after.data) {
			continue
		}
		action := "update"
		if !before.exists {
			action = "create"
		} else if !after.exists {
			action = "remove"
		}
		changes = append(changes, FileChange{Path: before.path, Action: action})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func (p *installPlan) apply() (*fileTransaction, error) {
	// A user can edit files while reviewing the preview. Do not overwrite them.
	for _, before := range p.before.files {
		check := &fileTransaction{}
		if err := check.remember(before.path); err != nil {
			return nil, err
		}
		now := check.files[0]
		if now.link != "" {
			var err error
			now.data, err = os.ReadFile(now.path)
			if err != nil {
				return nil, err
			}
		}
		if before.exists != now.exists || before.mode != now.mode || before.link != now.link || !bytes.Equal(before.data, now.data) {
			return nil, fmt.Errorf("installation file changed after preview; run install again: %s", before.path)
		}
	}
	tx := &fileTransaction{}
	for _, operation := range p.operations {
		if err := operation(tx); err != nil {
			tx.rollback()
			return nil, err
		}
	}
	return tx, nil
}
