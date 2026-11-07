package state

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"iter"
	"sort"
)

// BaselineRef identifies immutable Git metadata, not a mutable revision name.
// All IO that prepares a baseline belongs to adapters; State only reads memory.
type BaselineRef struct {
	CommonDir    string `json:"common_dir"`
	Tree         string `json:"tree"`
	ObjectFormat string `json:"object_format"`
}
type baseline struct {
	ref   BaselineRef
	files map[string]File
	paths []string
}

const deletedMode = "@deleted"

// FromBaseline takes an independent copy of the prepared Git tree metadata.
// The baseline has no Motif AVL nodes; only overrides allocate path nodes.
func FromBaseline(ref BaselineRef, files map[string]File) State {
	b := &baseline{ref: ref, files: make(map[string]File, len(files))}
	for p, f := range files {
		b.files[p] = f
		b.paths = append(b.paths, p)
	}
	sort.Strings(b.paths)
	return State{base: b}
}
func (s State) Baseline() (BaselineRef, bool) {
	if s.base == nil {
		return BaselineRef{}, false
	}
	return s.base.ref, true
}
func (s State) set(p string, f File) State {
	if s.base != nil {
		if original, ok := s.base.files[p]; ok && SameFile(original, f) {
			return State{root: remove(s.root, p), base: s.base}
		}
	}
	return State{root: put(s.root, p, f), base: s.base}
}
func (s State) delete(p string) State {
	if s.base != nil {
		if _, ok := s.base.files[p]; ok {
			return State{root: put(s.root, p, File{Mode: deletedMode}), base: s.base}
		}
	}
	return State{root: remove(s.root, p), base: s.base}
}
func (s State) lookup(p string) (File, bool) {
	if f, ok := (State{root: s.root}).Lookup(p); ok {
		if f.Mode == deletedMode {
			return File{}, false
		}
		return f, true
	}
	f, ok := s.base.files[p]
	return f, ok
}
func (s State) length() int {
	if s.base == nil {
		return count(s.root)
	}
	n := len(s.base.files)
	for p, f := range (State{root: s.root}).All() {
		_, inBase := s.base.files[p]
		if f.Mode == deletedMode {
			n--
		} else if !inBase {
			n++
		}
	}
	return n
}
func (s State) all() iter.Seq2[string, File] {
	return func(yield func(string, File) bool) {
		next, stop := iter.Pull2((State{root: s.root}).All())
		defer stop()
		op, of, ok := next()
		for _, p := range s.base.paths {
			for ok && op < p {
				if of.Mode != deletedMode && !yield(op, of) {
					return
				}
				op, of, ok = next()
			}
			if ok && op == p {
				if of.Mode != deletedMode && !yield(op, of) {
					return
				}
				op, of, ok = next()
			} else if !yield(p, s.base.files[p]) {
				return
			}
		}
		for ok {
			if of.Mode != deletedMode && !yield(op, of) {
				return
			}
			op, of, ok = next()
		}
	}
}
func sameBaseline(a, b State) bool { return a.base != nil && b.base != nil && a.base.ref == b.base.ref }
func changedSparse(a, b State) iter.Seq[string] {
	return func(yield func(string) bool) {
		if a.base == b.base && a.root == b.root {
			return
		}
		paths := map[string]bool{}
		if sameBaseline(a, b) {
			for p := range (State{root: a.root}).All() {
				paths[p] = true
			}
			for p := range (State{root: b.root}).All() {
				paths[p] = true
			}
		} else {
			for p := range a.All() {
				paths[p] = true
			}
			for p := range b.All() {
				paths[p] = true
			}
		}
		ordered := make([]string, 0, len(paths))
		for p := range paths {
			ordered = append(ordered, p)
		}
		sort.Strings(ordered)
		for _, p := range ordered {
			af, ak := a.Lookup(p)
			bf, bk := b.Lookup(p)
			if ak != bk || (ak && !SameFile(af, bf)) {
				if !yield(p) {
					return
				}
			}
		}
	}
}

// Overrides is the complete sparse overlay. A nil file is a deletion.
// Full states expose every entry, with an empty baseline on persistence.
type OverlayEntry struct {
	Path []byte `json:"path"`
	File *File  `json:"file"`
}

func (s State) Overrides() []OverlayEntry {
	entries := make([]OverlayEntry, 0, count(s.root))
	for p, f := range (State{root: s.root}).All() {
		e := OverlayEntry{Path: []byte(p)}
		if f.Mode != deletedMode {
			v := f
			e.File = &v
		}
		entries = append(entries, e)
	}
	return entries
}

type descriptor struct {
	Version int            `json:"version"`
	Base    BaselineRef    `json:"base"`
	Entries []OverlayEntry `json:"entries"`
}

func (s State) descriptor() []byte {
	raw, _ := json.Marshal(descriptor{2, s.base.ref, s.Overrides()})
	return raw
}
func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func restoreDescriptor(raw []byte, root string, cache map[string]State, load func(BaselineRef) (State, error)) (State, error) {
	var d descriptor
	if e := json.Unmarshal(raw, &d); e != nil {
		return State{}, e
	}
	if load == nil {
		return State{}, fmt.Errorf("Git baseline resolver required")
	}
	s, e := load(d.Base)
	if e != nil {
		return State{}, e
	}
	if ref, ok := s.Baseline(); !ok || ref != d.Base {
		return State{}, fmt.Errorf("baseline identity mismatch")
	}
	last := ""
	for i, entry := range d.Entries {
		p := string(entry.Path)
		if i > 0 && p <= last {
			return State{}, fmt.Errorf("unordered state overlay")
		}
		last = p
		if entry.File == nil {
			s = s.Delete(p)
		} else {
			s = s.Set(p, *entry.File)
		}
	}
	if s.Root() != root {
		return State{}, fmt.Errorf("non-canonical state descriptor")
	}
	// Do not retain every descriptor: its baseline may contain millions of paths.
	// The adapter bounds and shares baseline metadata independently.
	return s, nil
}
