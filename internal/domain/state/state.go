// Package state defines immutable, structurally shared file states.
package state

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"sort"
	"unicode/utf8"
)

type File struct {
	ContentID  string `json:"content_id,omitempty"`
	OutOfScope bool   `json:"out_of_scope,omitempty"`
	Mode       string `json:"mode"`
	ContentRef string `json:"content_ref,omitempty"`
	OID        string `json:"oid,omitempty"`
}

// State is a persistent AVL path index. A copy shares all unchanged nodes;
// Set/Delete copy only O(log(paths)) nodes and never mutate an earlier state.
// The zero value is the empty state.
type State struct{ root *node }
type node struct {
	path          string
	file          File
	left, right   *node
	height, count int
	id            string
}

func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}
func count(n *node) int {
	if n == nil {
		return 0
	}
	return n.count
}
func makeNode(p string, f File, l, r *node) *node {
	n := &node{path: p, file: f, left: l, right: r, height: 1 + max(height(l), height(r)), count: 1 + count(l) + count(r)}
	raw, _ := json.Marshal(n.record())
	n.id = fmt.Sprintf("%x", sha256.Sum256(raw))
	return n
}
func rotateRight(n *node) *node {
	l := n.left
	return makeNode(l.path, l.file, l.left, makeNode(n.path, n.file, l.right, n.right))
}
func rotateLeft(n *node) *node {
	r := n.right
	return makeNode(r.path, r.file, makeNode(n.path, n.file, n.left, r.left), r.right)
}
func balance(n *node) *node {
	if height(n.left)-height(n.right) > 1 {
		if height(n.left.left) < height(n.left.right) {
			n = makeNode(n.path, n.file, rotateLeft(n.left), n.right)
		}
		return rotateRight(n)
	}
	if height(n.right)-height(n.left) > 1 {
		if height(n.right.right) < height(n.right.left) {
			n = makeNode(n.path, n.file, n.left, rotateRight(n.right))
		}
		return rotateLeft(n)
	}
	return n
}
func put(n *node, p string, f File) *node {
	if n == nil {
		return makeNode(p, f, nil, nil)
	}
	if p == n.path {
		if f == n.file {
			return n
		}
		return makeNode(p, f, n.left, n.right)
	}
	if p < n.path {
		l := put(n.left, p, f)
		if l == n.left {
			return n
		}
		return balance(makeNode(n.path, n.file, l, n.right))
	}
	r := put(n.right, p, f)
	if r == n.right {
		return n
	}
	return balance(makeNode(n.path, n.file, n.left, r))
}
func remove(n *node, p string) *node {
	if n == nil {
		return nil
	}
	if p < n.path {
		l := remove(n.left, p)
		if l == n.left {
			return n
		}
		return balance(makeNode(n.path, n.file, l, n.right))
	}
	if p > n.path {
		r := remove(n.right, p)
		if r == n.right {
			return n
		}
		return balance(makeNode(n.path, n.file, n.left, r))
	}
	if n.left == nil {
		return n.right
	}
	if n.right == nil {
		return n.left
	}
	successor := n.right
	for successor.left != nil {
		successor = successor.left
	}
	return balance(makeNode(successor.path, successor.file, n.left, remove(n.right, successor.path)))
}
func (s State) Set(p string, f File) State { return State{put(s.root, p, f)} }
func (s State) Delete(p string) State      { return State{remove(s.root, p)} }
func (s State) Len() int                   { return count(s.root) }
func (s State) Lookup(p string) (File, bool) {
	for n := s.root; n != nil; {
		if p == n.path {
			return n.file, true
		}
		if p < n.path {
			n = n.left
		} else {
			n = n.right
		}
	}
	return File{}, false
}
func (s State) Get(p string) File { f, _ := s.Lookup(p); return f }
func (s State) All() iter.Seq2[string, File] {
	return func(yield func(string, File) bool) {
		var walk func(*node) bool
		walk = func(n *node) bool { return n == nil || (walk(n.left) && yield(n.path, n.file) && walk(n.right)) }
		walk(s.root)
	}
}
func From(m map[string]File) State {
	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var build func(int, int) *node
	build = func(a, b int) *node {
		if a >= b {
			return nil
		}
		mid := (a + b) / 2
		p := paths[mid]
		return makeNode(p, m[p], build(a, mid), build(mid+1, b))
	}
	return State{build(0, len(paths))}
}
func Clone(s State) State { return s }
func Equal(a, b State) bool {
	if a.root == b.root {
		return true
	}
	if a.Len() != b.Len() {
		return false
	}
	for p, f := range a.All() {
		if g, ok := b.Lookup(p); !ok || !SameFile(g, f) {
			return false
		}
	}
	return true
}
func (s State) MarshalJSON() ([]byte, error) {
	m := map[string]File{}
	for p, f := range s.All() {
		m[base64.StdEncoding.EncodeToString([]byte(p))] = f
	}
	return json.Marshal(m)
}
func (s *State) UnmarshalJSON(raw []byte) error {
	var m map[string]File
	if e := json.Unmarshal(raw, &m); e != nil {
		return e
	}
	out := State{}
	for p, f := range m {
		b, e := base64.StdEncoding.DecodeString(p)
		if e != nil {
			return e
		}
		out = out.Set(string(b), f)
	}
	*s = out
	return nil
}
func DisplayPath(p string) string {
	if utf8.ValidString(p) {
		return p
	}
	return fmt.Sprintf("%q", []byte(p))
}
func Apply(current, before, after State) (State, error) {
	out := current
	for p := range ChangedPaths(before, after) {
		b, bok := before.Lookup(p)
		a, aok := after.Lookup(p)
		c, cok := current.Lookup(p)
		if bok != cok || !SameFile(b, c) {
			return State{}, fmt.Errorf("cannot replay path %s: recorded before state differs", DisplayPath(p))
		}
		if aok {
			out = out.Set(p, a)
		} else {
			out = out.Delete(p)
		}
	}
	return out, nil
}

// ChangedPaths avoids traversing shared subtrees when their roots agree.
func ChangedPaths(a, b State) iter.Seq[string] {
	return func(yield func(string) bool) {
		if a.root == b.root {
			return
		}
		// Aligned subtrees can be skipped; rotations fall back to ordered comparison.
		var walk func(*node, *node) bool
		walk = func(x, y *node) bool {
			if x == y {
				return true
			}
			if x != nil && y != nil && x.path == y.path {
				if !walk(x.left, y.left) {
					return false
				}
				if !SameFile(x.file, y.file) && !yield(x.path) {
					return false
				}
				return walk(x.right, y.right)
			}
			as, stopA := iter.Pull2((State{x}).All())
			defer stopA()
			bs, stopB := iter.Pull2((State{y}).All())
			defer stopB()
			ap, af, aok := as()
			bp, bf, bok := bs()
			for aok || bok {
				if !bok || (aok && ap < bp) {
					if !yield(ap) {
						return false
					}
					ap, af, aok = as()
				} else if !aok || bp < ap {
					if !yield(bp) {
						return false
					}
					bp, bf, bok = bs()
				} else {
					if !SameFile(af, bf) && !yield(ap) {
						return false
					}
					ap, af, aok = as()
					bp, bf, bok = bs()
				}
			}
			return true
		}
		walk(a.root, b.root)
	}
}

// NodeRecord is the versioned durable representation of one shared path node.
type NodeRecord struct {
	Version     int    `json:"version"`
	Path        []byte `json:"path"`
	File        File   `json:"file"`
	Left, Right string
}

func id(n *node) string {
	if n == nil {
		return ""
	}
	return n.id
}
func (n *node) record() NodeRecord {
	return NodeRecord{1, []byte(n.path), n.file, id(n.left), id(n.right)}
}
func (s State) Root() string { return id(s.root) }

// Persist stops at nodes already durable; no complete snapshot serialization.
func (s State) Persist(exists func(string) bool, write func(string, []byte) error) error {
	var walk func(*node) error
	walk = func(n *node) error {
		if n == nil || exists(n.id) {
			return nil
		}
		if e := walk(n.left); e != nil {
			return e
		}
		if e := walk(n.right); e != nil {
			return e
		}
		raw, _ := json.Marshal(n.record())
		return write(n.id, raw)
	}
	return walk(s.root)
}

// Restore uses the caller's bounded node cache to share nodes across snapshots.
func Restore(root string, read func(string) ([]byte, error), cache map[string]State) (State, error) {
	if root == "" {
		return State{}, nil
	}
	if s, ok := cache[root]; ok {
		return s, nil
	}
	raw, e := read(root)
	if e != nil {
		return State{}, e
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != root {
		return State{}, fmt.Errorf("corrupt state node %s", root)
	}
	var r NodeRecord
	if e = json.Unmarshal(raw, &r); e != nil {
		return State{}, e
	}
	if r.Version != 1 {
		return State{}, fmt.Errorf("unsupported state node version")
	}
	l, e := Restore(r.Left, read, cache)
	if e != nil {
		return State{}, e
	}
	right, e := Restore(r.Right, read, cache)
	if e != nil {
		return State{}, e
	}
	n := makeNode(string(r.Path), r.File, l.root, right.root)
	s := State{n}
	if len(cache) < 100000 {
		cache[root] = s
	}
	return s, nil
}

// SameFile compares semantic content identity separately from its storage location.
func SameFile(a, b File) bool {
	if a.Mode != b.Mode || a.OID != b.OID || a.OutOfScope != b.OutOfScope {
		return false
	}
	if a.ContentRef == b.ContentRef {
		return true
	}
	return a.ContentID != "" && a.ContentID == b.ContentID
}
