package gitrepo_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/gitrepo"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/capture"
	"github.com/yhlooo/motif/internal/app/workspace"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
)

type captureFixture struct {
	t       testing.TB
	root    string
	r       *gitrepo.Repository
	objects content.Objects
}

func newCaptureFixture(t testing.TB) *captureFixture {
	t.Helper()
	f := &captureFixture{t: t, root: t.TempDir(), objects: content.Objects{Dir: t.TempDir()}}
	f.git("init", "-q")
	f.git("config", "user.name", "test")
	f.git("config", "user.email", "test@example.com")
	var e error
	f.r, e = gitrepo.Open(context.Background(), f.root, f.objects)
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func (f *captureFixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.root
	raw, e := cmd.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %s %v", args, raw, e)
	}
	return string(raw)
}
func (f *captureFixture) write(path, value string) {
	f.t.Helper()
	full := filepath.Join(f.root, path)
	if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
		f.t.Fatal(e)
	}
	if e := os.WriteFile(full, []byte(value), 0600); e != nil {
		f.t.Fatal(e)
	}
}
func (f *captureFixture) base() domain.State {
	f.t.Helper()
	head, e := f.r.Head(context.Background())
	if e != nil {
		f.t.Fatal(e)
	}
	st, e := f.r.Tree(context.Background(), head)
	if e != nil {
		f.t.Fatal(e)
	}
	return st
}
func (f *captureFixture) capture(base domain.State, prev ...domain.State) domain.State {
	f.t.Helper()
	st, e := f.r.Worktree(context.Background(), base, prev...)
	if e != nil {
		f.t.Fatal(e)
	}
	return st
}
func (f *captureFixture) expect(st domain.State, path, value string) {
	f.t.Helper()
	entry, ok := st.Lookup(path)
	if !ok {
		f.t.Fatal("missing", path)
	}
	raw, e := f.objects.Read(context.Background(), entry.ContentRef)
	if e != nil || string(raw) != value {
		f.t.Fatalf("%q: got %q want %q: %v", path, raw, value, e)
	}
}

func TestCaptureCleanBaselineDoesNotReadOrPersistWholeTree(t *testing.T) {
	f := newCaptureFixture(t)
	for i := 0; i < 300; i++ {
		f.write(fmt.Sprintf("dir/%04d", i), "committed bytes")
	}
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	st := f.capture(base)
	if f.r.ReadStats().WorktreeBytes != 0 || len(st.Overrides()) != 0 || !domain.Equal(base, st) {
		t.Fatal("clean contents read or overridden", f.r.ReadStats())
	}
	writes := 0
	if e := st.Persist(func(string) bool { return false }, func(_ string, raw []byte) error {
		writes++
		if len(raw) > 1000 {
			t.Fatal("full baseline persisted")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if writes != 1 {
		t.Fatal(writes)
	}
	s, e := sqlite.Open(f.objects.Dir)
	if e != nil {
		t.Fatal(e)
	}
	w, e := s.Register(context.Background(), f.r.CommonDir, f.root, f.r.GitDir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(context.Background(), domain.Observation{Worktree: w, Trigger: "Stop", BaseState: base, State: st}); e != nil {
		t.Fatal(e)
	}
	c, e := s.Cursor(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = sqlite.Open(f.objects.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	saved, e := s.Snapshot(c.SnapshotID)
	if e != nil || !domain.Equal(st, saved.State) {
		t.Fatal("reopen sparse snapshot", e)
	}
	f.expect(saved.State, "dir/0000", "committed bytes")
}
func TestCaptureCandidatesPreserveActualBytesAndScope(t *testing.T) {
	f := newCaptureFixture(t)
	f.write("normal", "base")
	f.write("assumed", "base")
	f.write("filtered", "HELLO\n")
	f.write("removed-index", "base")
	f.write("mode", "base")
	f.write(".gitattributes", "filtered filter=upper\n")
	f.git("config", "filter.upper.clean", "tr a-z A-Z")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	f.write("normal", "staged")
	f.git("add", "normal")
	f.write("normal", "actual worktree")
	f.write("filtered", "hello\n")
	f.git("update-index", "--assume-unchanged", "assumed")
	f.write("assumed", "hidden edit")
	f.git("rm", "--cached", "removed-index")
	f.write(".gitignore", "removed-index\n")
	f.git("config", "core.filemode", "false")
	if e := os.Chmod(filepath.Join(f.root, "mode"), 0700); e != nil {
		t.Fatal(e)
	}
	f.write("new/\xff\nfile", "untracked")
	st := f.capture(base)
	for path, want := range map[string]string{"normal": "actual worktree", "assumed": "hidden edit", "filtered": "hello\n", "removed-index": "base", "new/\xff\nfile": "untracked"} {
		f.expect(st, path, want)
	}
	if st.Get("mode").Mode != "100755" {
		t.Fatal("mode ignored")
	}
	f.write(".gitignore", "removed-index\nnew/\n")
	next := f.capture(base, st)
	if entry, ok := next.Lookup("new/\xff\nfile"); !ok || !entry.OutOfScope || entry.ContentRef != "" {
		t.Fatal("scope exit lost", entry)
	}
	f.write(".gitignore", "removed-index\n")
	again := f.capture(base, next)
	f.expect(again, "new/\xff\nfile", "untracked")
	if e := os.Remove(filepath.Join(f.root, "normal")); e != nil {
		t.Fatal(e)
	}
	deleted := f.capture(base, again)
	if _, ok := deleted.Lookup("normal"); ok {
		t.Fatal("deletion lost")
	}
	f.write("normal", "base")
	f.git("reset", "-q", "HEAD", "normal")
	restored := f.capture(base, deleted)
	f.expect(restored, "normal", "base")
}
func TestCaptureBatchScopeExitHasBoundedGitCalls(t *testing.T) {
	f := newCaptureFixture(t)
	f.write("tracked", "base")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	for i := 0; i < 80; i++ {
		f.write(fmt.Sprintf("generated/%d", i), "bytes")
	}
	st := f.capture(base)
	f.write(".gitignore", "generated/\n")
	before := f.r.ReadStats()
	next := f.capture(base, st)
	calls := f.r.ReadStats().GitCalls - before.GitCalls
	if calls > 10 {
		t.Fatal("per-path Git processes", calls)
	}
	for i := 0; i < 80; i++ {
		if !next.Get(fmt.Sprintf("generated/%d", i)).OutOfScope {
			t.Fatal("scope missing")
		}
	}
}

func TestCaptureDirectoryCacheDoesNotFollowAncestorSymlink(t *testing.T) {
	f := newCaptureFixture(t)
	f.write("inside/nested/a", "base")
	f.write("inside/nested/b", "base")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	outside := t.TempDir()
	if e := os.MkdirAll(filepath.Join(outside, "nested"), 0700); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"a", "b"} {
		if e := os.WriteFile(filepath.Join(outside, "nested", p), []byte("outside"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.RemoveAll(filepath.Join(f.root, "inside")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(outside, filepath.Join(f.root, "inside")); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 5; i++ {
		st := f.capture(base)
		for _, p := range []string{"inside/nested/a", "inside/nested/b"} {
			if _, ok := st.Lookup(p); ok {
				t.Fatal("unsafe ancestor inherited or captured", p)
			}
		}
	}
}

func TestCaptureDoesNotRunCleanFilterToFindCandidates(t *testing.T) {
	f := newCaptureFixture(t)
	f.write(".gitattributes", "file filter=upper\n")
	f.git("config", "filter.upper.clean", "tr a-z A-Z")
	f.write("file", "HELLO\n")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	f.git("config", "filter.upper.required", "true")
	f.git("config", "filter.upper.clean", "false")
	f.write("file", "changed raw bytes\n")
	st := f.capture(base)
	f.expect(st, "file", "changed raw bytes\n")
}
func TestLegacySnapshotCanBeFollowedBySparseSnapshot(t *testing.T) {
	f := newCaptureFixture(t)
	f.write("file", "base")
	f.git("add", ".")
	f.git("commit", "-qm", "base")
	base := f.base()
	files := map[string]domain.File{}
	for p, entry := range base.All() {
		files[p] = entry
	}
	legacy := domain.From(files)
	s, e := sqlite.Open(f.objects.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	project, e := workspace.Open(ctx, f.root, s)
	if e != nil {
		t.Fatal(e)
	}
	w, e := project.Identity(ctx)
	if e != nil {
		t.Fatal(e)
	}
	head, e := project.Head(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Observe(ctx, domain.Observation{Worktree: w, Base: head, BaseState: legacy, State: legacy, Trigger: "Stop"}); e != nil {
		t.Fatal(e)
	}
	old, e := s.Cursor(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = capture.Observe(ctx, project, w, domain.Observation{Trigger: "Stop"}); e != nil {
		t.Fatal(e)
	}
	current, e := s.Cursor(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := s.Snapshot(current.SnapshotID)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := saved.State.Baseline(); !ok || !domain.Equal(legacy, saved.State) {
		t.Fatal("upgrade lost state")
	}
	before, e := s.Snapshot(old.SnapshotID)
	if e != nil || !domain.Equal(before.State, legacy) {
		t.Fatal("legacy history changed", e)
	}
}

func BenchmarkSparseWorktree(b *testing.B) {
	for _, n := range []int{1000, 30000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			f := newCaptureFixture(b)
			for i := 0; i < n; i++ {
				f.write(fmt.Sprintf("dir/%05d", i), "committed bytes")
			}
			f.git("add", ".")
			f.git("commit", "-qm", "base")
			head, e := f.r.Head(context.Background())
			if e != nil {
				b.Fatal(e)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r, e := gitrepo.Open(context.Background(), f.root, f.objects)
				if e != nil {
					b.Fatal(e)
				}
				base, e := r.Tree(context.Background(), head)
				if e != nil {
					b.Fatal(e)
				}
				st, e := r.Worktree(context.Background(), base)
				if e != nil {
					b.Fatal(e)
				}
				if r.ReadStats().WorktreeBytes != 0 || len(st.Overrides()) != 0 {
					b.Fatal("clean baseline read or copied")
				}
			}
			b.ReportMetric(0, "worktree-bytes/op")
		})
	}
}
