package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
)

func TestPersistentStatesAgainstMap(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	s := State{}
	model := map[string]File{}
	history := []State{}
	references := []map[string]File{}
	for step := 0; step < 2000; step++ {
		path := fmt.Sprintf("dir/%03d/\xff", rng.Intn(150))
		if rng.Intn(4) == 0 {
			s = s.Delete(path)
			delete(model, path)
		} else {
			f := File{Mode: "100644", ContentRef: fmt.Sprint(rng.Intn(50))}
			s = s.Set(path, f)
			model[path] = f
		}
		if step%100 == 0 {
			copy := map[string]File{}
			for p, f := range model {
				copy[p] = f
			}
			history = append(history, s)
			references = append(references, copy)
		}
		if s.Len() != len(model) {
			t.Fatal("size mismatch")
		}
		for p, f := range model {
			if got, ok := s.Lookup(p); !ok || got != f {
				t.Fatal("lookup mismatch")
			}
		}
	}
	for i, old := range history {
		if !Equal(old, From(references[i])) {
			t.Fatal("history mutated")
		}
	}
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var roundtrip State
	if e = json.Unmarshal(raw, &roundtrip); e != nil || !Equal(s, roundtrip) {
		t.Fatal("raw path JSON changed", e)
	}
}
func TestDurableNodesShareStructureAndDetectCorruption(t *testing.T) {
	s := State{}
	for i := 0; i < 1000; i++ {
		s = s.Set(fmt.Sprintf("%04d", i), File{Mode: "100644", ContentRef: fmt.Sprint(i)})
	}
	objects := map[string][]byte{}
	persist := func(s State) int {
		before := len(objects)
		e := s.Persist(func(id string) bool { _, ok := objects[id]; return ok }, func(id string, data []byte) error { objects[id] = bytes.Clone(data); return nil })
		if e != nil {
			t.Fatal(e)
		}
		return len(objects) - before
	}
	if n := persist(s); n != 1000 {
		t.Fatal("unexpected initial nodes", n)
	}
	next := s.Set("0500", File{Mode: "100755", ContentRef: "modified"})
	if n := persist(next); n > 20 {
		t.Fatal("single-path update copied too many nodes", n)
	}
	read := func(id string) ([]byte, error) {
		raw, ok := objects[id]
		if !ok {
			return nil, fmt.Errorf("missing node")
		}
		return raw, nil
	}
	restored, e := Restore(next.Root(), read, map[string]State{})
	if e != nil || !Equal(next, restored) {
		t.Fatal("restart changed state", e)
	}
	if s.Get("0500").Mode != "100644" {
		t.Fatal("old root mutated")
	}
	objects[next.Root()] = []byte("corrupt")
	if _, e = Restore(next.Root(), read, map[string]State{}); e == nil {
		t.Fatal("corrupt node accepted")
	}
}
func TestChangedPathsIgnoresStorageLocationButNotMode(t *testing.T) {
	a := From(map[string]File{"same": {Mode: "100644", ContentRef: "local", ContentID: "sha1:identity"}})
	b := a.Set("same", File{Mode: "100644", ContentRef: "git:reference", ContentID: "sha1:identity"})
	if !Equal(a, b) {
		t.Fatal("same bytes treated as new change")
	}
	for range ChangedPaths(a, b) {
		t.Fatal("false delta")
	}
	b = b.Set("same", File{Mode: "100755", ContentRef: "git:reference", ContentID: "sha1:identity"})
	n := 0
	for p := range ChangedPaths(a, b) {
		n++
		if p != "same" {
			t.Fatal(p)
		}
	}
	if n != 1 {
		t.Fatal(n)
	}
}
func BenchmarkStateSinglePathUpdate(b *testing.B) {
	for _, n := range []int{100, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := State{}
			for i := 0; i < n; i++ {
				s = s.Set(fmt.Sprint(i), File{Mode: "100644", ContentRef: "original"})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				next := s.Set("50", File{Mode: "100644", ContentRef: "changed"})
				if next.Len() != n {
					b.Fatal("lost entries")
				}
			}
		})
	}
}
