package gitrepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/yhlooo/motif/pkg/gitrepo"
	"github.com/yhlooo/motif/pkg/store"
)

func TestWorktreeActualBytesAndFileTypes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%s: %v", b, e)
		}
	}
	write := func(p string, b []byte) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, p), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Test")
	git("config", "user.email", "t@example.com")
	write("tracked", []byte("base\n"))
	write("deleted", []byte("base\n"))
	write("dir/child", []byte("inside\n"))
	write("staged-delete", []byte("keep on disk\n"))
	write(".gitignore", []byte("ignored\ntracked\n"))
	git("add", "-f", "tracked")
	git("add", ".")
	git("commit", "-m", "base")
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	head, e := r.Head(ctx)
	if e != nil {
		t.Fatal(e)
	}
	base, e := r.Tree(ctx, head)
	if e != nil {
		t.Fatal(e)
	}
	git("rm", "--cached", "staged-delete")
	write("tracked", []byte("staged\n"))
	git("add", "-f", "tracked")
	write("tracked", []byte("actual worktree\n"))
	write("ignored", []byte("never capture\n"))
	write("empty", nil)
	write("binary", []byte{0, 1, 255})
	write("executable", []byte("#!/bin/sh\n"))
	if e = os.Chmod(filepath.Join(root, "executable"), 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(root, "deleted")); e != nil {
		t.Fatal(e)
	}
	outside := t.TempDir()
	if e = os.WriteFile(filepath.Join(outside, "child"), []byte("secret"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.RemoveAll(filepath.Join(root, "dir")); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, filepath.Join(root, "dir")); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink("tracked", filepath.Join(root, "link")); e != nil {
		t.Fatal(e)
	}
	rawPath := string([]byte{'p', 255})
	write(rawPath, []byte("raw path\n"))
	state, e := r.Worktree(ctx, base)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := state["ignored"]; ok {
		t.Fatal("ignored path captured")
	}
	if _, ok := state["dir/child"]; ok {
		t.Fatal("followed external ancestor symlink")
	}
	if _, ok := state["deleted"]; ok {
		t.Fatal("deleted path captured")
	}
	if _, ok := state["staged-delete"]; !ok {
		t.Fatal("index used in place of worktree")
	}
	if state["dir"].Mode != "120000" || state["link"].Mode != "120000" || state["executable"].Mode != "100755" {
		t.Fatal("file modes lost")
	}
	data, e := s.Blob(state["tracked"].Hash)
	if e != nil || string(data) != "actual worktree\n" {
		t.Fatal("not actual worktree bytes", e)
	}
	empty, e := s.Blob(state["empty"].Hash)
	if e != nil || len(empty) != 0 {
		t.Fatal("empty file mistaken for delete", e)
	}
	encoded, e := json.Marshal(state)
	if e != nil {
		t.Fatal(e)
	}
	var decoded store.State
	if e = json.Unmarshal(encoded, &decoded); e != nil || !store.Equal(state, decoded) {
		t.Fatal("path encoding was not reversible", e)
	}
	diff, e := r.Diff(ctx, base, state)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, d := range diff {
		if d.Path == "binary" {
			found = d.Binary
		}
	}
	if !found {
		t.Fatal("binary summary missing")
	}
}

func TestIgnoredPathScopeTransition(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("%s %v", b, e)
		}
	}
	git("init")
	git("config", "user.name", "Test")
	git("config", "user.email", "t@example.com")
	if e := os.WriteFile(filepath.Join(root, "tracked"), []byte("base"), 0644); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "base")
	s, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := gitrepo.Open(ctx, root, s)
	if e != nil {
		t.Fatal(e)
	}
	head, e := r.Head(ctx)
	if e != nil {
		t.Fatal(e)
	}
	base, e := r.Tree(ctx, head)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "temporary"), []byte("first"), 0644); e != nil {
		t.Fatal(e)
	}
	first, e := r.Worktree(ctx, base)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("temporary\n"), 0644); e != nil {
		t.Fatal(e)
	}
	second, e := r.Worktree(ctx, base, first)
	if e != nil {
		t.Fatal(e)
	}
	f, ok := second["temporary"]
	if !ok || !f.OutOfScope || f.Hash != "" {
		t.Fatal("path leaving scope was treated as deletion or content was kept")
	}
	if e = os.WriteFile(filepath.Join(root, "temporary"), []byte("uncaptured"), 0644); e != nil {
		t.Fatal(e)
	}
	third, e := r.Worktree(ctx, base, second)
	if e != nil || third["temporary"] != f {
		t.Fatal("out-of-scope bytes were captured", e)
	}
	if e = os.Remove(filepath.Join(root, ".gitignore")); e != nil {
		t.Fatal(e)
	}
	last, e := r.Worktree(ctx, base, third)
	if e != nil || last["temporary"].OutOfScope || last["temporary"].Hash == "" {
		t.Fatal("path re-entry lost", e)
	}
}

func BenchmarkTreeRead(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			ctx := context.Background()
			root := b.TempDir()
			git := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				if raw, err := cmd.CombinedOutput(); err != nil {
					b.Fatalf("%s: %v", raw, err)
				}
			}
			git("init", "-b", "main")
			git("config", "user.name", "Bench")
			git("config", "user.email", "bench@example.com")
			for i := 0; i < size; i++ {
				if err := os.WriteFile(filepath.Join(root, strconv.Itoa(i)), []byte(fmt.Sprintf("file %d\n", i)), 0644); err != nil {
					b.Fatal(err)
				}
			}
			git("add", ".")
			git("commit", "-m", "base")
			s, err := store.Open(b.TempDir())
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			r, err := gitrepo.Open(ctx, root, s)
			if err != nil {
				b.Fatal(err)
			}
			head, err := r.Head(ctx)
			if err != nil {
				b.Fatal(err)
			}
			b.Run("cold", func(b *testing.B) {
				var calls int64
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					cold := &gitrepo.Repository{Root: root, CommonDir: r.CommonDir, Store: s}
					state, err := cold.Tree(ctx, head)
					if err != nil || len(state) != size {
						b.Fatal(err)
					}
					calls += cold.ReadStats().GitCalls
				}
				b.ReportMetric(float64(calls)/float64(b.N), "git/op")
			})
			if _, err = r.Tree(ctx, head); err != nil {
				b.Fatal(err)
			}
			b.Run("warm", func(b *testing.B) {
				before := r.ReadStats()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					state, err := r.Tree(ctx, head)
					if err != nil || len(state) != size {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(r.ReadStats().GitCalls-before.GitCalls)/float64(b.N), "git/op")
			})
		})
	}
}
