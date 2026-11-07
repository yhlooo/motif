package state

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

func TestSparseBaselineMatchesFullState(t *testing.T) {
	files := map[string]File{}
	for i := 0; i < 100; i++ {
		files[fmt.Sprintf("%03d/\xff", i)] = File{Mode: "100644", ContentRef: fmt.Sprint(i)}
	}
	ref := BaselineRef{CommonDir: "/repo.git", Tree: "tree", ObjectFormat: "sha1"}
	base := FromBaseline(ref, files)
	s := base
	full := From(files)
	rng := rand.New(rand.NewSource(61))
	for step := 0; step < 500; step++ {
		old, oldFull := s, full
		p := fmt.Sprintf("%03d/\xff", rng.Intn(150))
		if rng.Intn(3) == 0 {
			s = s.Delete(p)
			full = full.Delete(p)
		} else {
			f := File{Mode: "100644", ContentRef: fmt.Sprint(rng.Intn(100))}
			s = s.Set(p, f)
			full = full.Set(p, f)
		}
		if s.Len() != full.Len() || !Equal(s, full) {
			t.Fatal("sparse/full mismatch", step)
		}
		a, b := []string{}, []string{}
		for p := range ChangedPaths(old, s) {
			a = append(a, p)
		}
		for p := range ChangedPaths(oldFull, full) {
			b = append(b, p)
		}
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatal("changed paths mismatch", a, b)
		}
	}
	if !Equal(base, From(files)) {
		t.Fatal("baseline mutated")
	}
	if !Equal(s, FromBaseline(BaselineRef{Tree: "other"}, toMap(full))) {
		t.Fatal("different bases cannot compare semantically")
	}
}
func toMap(s State) map[string]File {
	m := map[string]File{}
	for p, f := range s.All() {
		m[p] = f
	}
	return m
}
func TestSparseDescriptorIsSmallAndSelfContained(t *testing.T) {
	files := map[string]File{}
	for i := 0; i < 30000; i++ {
		files[fmt.Sprint(i)] = File{Mode: "100644", ContentRef: "git:blob"}
	}
	ref := BaselineRef{CommonDir: "/repo.git", Tree: "tree", ObjectFormat: "sha1"}
	base := FromBaseline(ref, files)
	next := base.Delete("0").Set("new/\xff", File{Mode: "100755", ContentRef: "local"})
	objects := map[string][]byte{}
	writes := 0
	if e := next.Persist(func(string) bool { return false }, func(id string, raw []byte) error { objects[id] = bytes.Clone(raw); writes++; return nil }); e != nil {
		t.Fatal(e)
	}
	if writes != 1 || len(objects[next.Root()]) > 1000 {
		t.Fatal("baseline serialized", writes, len(objects[next.Root()]))
	}
	read := func(id string) ([]byte, error) { return objects[id], nil }
	restored, e := RestoreWithBaseline(next.Root(), read, map[string]State{}, func(r BaselineRef) (State, error) {
		if r != ref {
			t.Fatal(r)
		}
		return FromBaseline(r, files), nil
	})
	if e != nil || !Equal(next, restored) || restored.Root() != next.Root() {
		t.Fatal("roundtrip", e)
	}
	if _, ok := restored.Lookup("0"); ok {
		t.Fatal("deletion lost")
	}
	if next.Set("0", files["0"]).Delete("new/\xff").Root() != base.Root() {
		t.Fatal("overlay did not normalize")
	}
	objects[next.Root()][0] ^= 1
	if _, e = RestoreWithBaseline(next.Root(), read, map[string]State{}, func(BaselineRef) (State, error) { return base, nil }); e == nil {
		t.Fatal("corruption accepted")
	}
}
