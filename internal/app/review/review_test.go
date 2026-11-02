package review_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yhlooo/motif/internal/app/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/capture"
	"github.com/yhlooo/motif/internal/app/review"
	"github.com/yhlooo/motif/internal/domain"
)

type fixture struct {
	t       testing.TB
	ctx     context.Context
	dir     string
	s       *store.Store
	r       *workspace.Project
	w       domain.Worktree
	service *review.Service
}

func setup(t testing.TB) *fixture {
	t.Helper()
	f := &fixture{t: t, ctx: context.Background(), dir: t.TempDir()}
	f.git("init", "-b", "main")
	f.git("config", "user.name", "Reviewer")
	f.git("config", "user.email", "reviewer@example.com")
	f.write("file.txt", "zero\n")
	f.git("add", ".")
	f.git("commit", "-m", "base")
	f.git("switch", "-c", "feature")
	var err error
	f.s, err = store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.s.Close() })
	f.r, err = workspace.Open(f.ctx, f.dir, f.s)
	if err != nil {
		t.Fatal(err)
	}
	f.w, err = f.r.Identity(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.service = &review.Service{Repo: f.r, Worktree: f.w}
	return f
}
func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %s %v", args, raw, err)
	}
	return strings.TrimSpace(string(raw))
}
func (f *fixture) write(p, value string) {
	f.t.Helper()
	path := filepath.Join(f.dir, p)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) hook(event, id string) {
	f.t.Helper()
	raw, _ := json.Marshal(capture.CodexInput{Event: event, CWD: f.dir, Session: "session", Turn: "turn", ToolID: id, Tool: "Edit", Input: json.RawMessage(`{"description":"original reason"}`)})
	if _, err := capture.Codex(f.ctx, f.s, raw); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) edit(id, p, content string) domain.Batch {
	f.t.Helper()
	f.hook("PreToolUse", id)
	f.write(p, content)
	f.hook("PostToolUse", id)
	events, err := f.s.Events(f.w.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	ids := []string{}
	for _, e := range events {
		if e.Kind == "tool_edit" && e.BatchID == "" {
			ids = append(ids, e.ID)
		}
	}
	batch, err := f.s.CreateBatch(f.ctx, f.w.ID, id, "intent "+id, ids)
	if err != nil {
		f.t.Fatal(err)
	}
	return batch
}
func (f *fixture) commit() string {
	f.t.Helper()
	f.git("add", ".")
	f.git("commit", "-m", "change")
	if err := capture.Git(f.ctx, f.s, f.dir, "post-commit"); err != nil {
		f.t.Fatal(err)
	}
	return f.git("rev-parse", "HEAD")
}
func (f *fixture) create() domain.Review {
	f.t.Helper()
	p, err := f.service.Preview(f.ctx, "feature", "main")
	if err != nil {
		f.t.Fatal(err)
	}
	r, err := f.service.Create(f.ctx, p, "Review")
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}
func (f *fixture) refresh(id string) domain.Review {
	f.t.Helper()
	_, err := f.service.Refresh(f.ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	r, err := f.s.Review(f.w.RepositoryID, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}
func TestRefreshStableIdentitiesAndRebatch(t *testing.T) {
	f := setup(t)
	batch := f.edit("first", "file.txt", "zero\nalpha\n")
	firstCommit := f.commit()
	r := f.create()
	original := r.Items[0]
	if err := f.s.Decision(f.ctx, f.w.RepositoryID, r.ID, original.ID, "approved", r.Version); err != nil {
		t.Fatal(err)
	}
	thread := domain.Thread{First: original.ID, Last: original.ID, Path: base64.StdEncoding.EncodeToString([]byte("file.txt")), Side: "new", Start: 2, End: 2}
	if err := f.service.AddThread(f.ctx, r, thread, "comment", "Reviewer"); err != nil {
		t.Fatal(err)
	}
	unchanged := f.refresh(r.ID)
	if unchanged.Version != r.Version || unchanged.Status != "approved" || unchanged.Items[0].ID != original.ID {
		t.Fatalf("unchanged view lost identity: %+v", unchanged)
	}
	f.edit("second", "new.txt", "new\n")
	head := f.commit()
	next := f.refresh(r.ID)
	if next.Head != head || next.Version <= r.Version || next.Status != "in_review" || len(next.Items) != 2 || next.Items[0].ID != original.ID || next.Items[0].Decision != "approved" || next.Items[1].Decision != "pending" {
		t.Fatalf("refresh failed: %+v", next)
	}
	if err := f.s.Decision(f.ctx, f.w.RepositoryID, r.ID, original.ID, "rejected", r.Version); !errors.Is(err, store.ErrStaleView) {
		t.Fatal("stale approval accepted", err)
	}
	comments, err := f.service.MappedThreads(f.ctx, next, next.Items[0].ID, next.Items[1].ID)
	if err != nil || len(comments) != 1 {
		t.Fatal("comment lost on extension", err)
	}
	if err = f.s.DeleteBatch(f.ctx, f.w.ID, batch.ID); err != nil {
		t.Fatal(err)
	}
	changed := f.refresh(r.ID)
	if changed.Items[0].ID == original.ID || changed.Items[0].Decision != "pending" || changed.Items[0].Kind != "unbatched_patch" {
		t.Fatalf("deleted batch retained approval: %+v", changed.Items[0])
	}
	comments, err = f.service.MappedThreads(f.ctx, changed, changed.Items[0].ID, changed.Items[1].ID)
	if err != nil || len(comments) != 0 {
		t.Fatal("deleted batch comment moved to another identity", err)
	}
	saved, err := f.s.Threads(f.w.RepositoryID, r.ID)
	if err != nil || len(saved) != 1 {
		t.Fatal("historical comment was deleted", err)
	}
	if _, err = f.s.CreateBatch(f.ctx, f.w.ID, "replacement", "new intent", batch.Members); err != nil {
		t.Fatal(err)
	}
	rebuilt := f.refresh(r.ID)
	if rebuilt.Items[0].ID == original.ID || rebuilt.Items[0].Decision != "pending" {
		t.Fatal("rebuilt batch inherited approval")
	}
	f.git("branch", "-f", "main", firstCommit)
	advanced := f.refresh(r.ID)
	if len(advanced.Items) != 1 || advanced.Items[0].ID != next.Items[1].ID {
		t.Fatal("target movement did not update range")
	}
	f.git("branch", "-f", "main", head)
	empty := f.refresh(r.ID)
	if len(empty.Items) != 0 || empty.Status == "approved" {
		t.Fatal("empty range retains invisible approvals")
	}
}
func TestCommentsMapInsertedDeletedAndRenamedLines(t *testing.T) {
	f := setup(t)
	f.edit("add", "file.txt", "zero\nalpha\nbeta\n")
	f.edit("insert", "file.txt", "header\nzero\nalpha\nbeta\n")
	f.edit("delete", "file.txt", "header\nzero\nbeta\n")
	f.commit()
	r := f.create()
	comment := domain.Thread{First: r.Items[0].ID, Last: r.Items[0].ID, Path: base64.StdEncoding.EncodeToString([]byte("file.txt")), Side: "new", Start: 2, End: 3}
	if err := f.service.AddThread(f.ctx, r, comment, "range", "Reviewer"); err != nil {
		t.Fatal(err)
	}
	mapped, err := f.service.MappedThreads(f.ctx, r, r.Items[0].ID, r.Items[1].ID)
	if err != nil || len(mapped) != 1 || mapped[0].Ranges[0] != (review.LineRange{Start: 3, End: 4}) {
		t.Fatalf("insertion mapping: %+v %v", mapped, err)
	}
	mapped, err = f.service.MappedThreads(f.ctx, r, r.Items[0].ID, r.Items[2].ID)
	if err != nil || len(mapped) != 1 || len(mapped[0].Ranges) != 1 || mapped[0].Ranges[0] != (review.LineRange{Start: 3, End: 3}) {
		t.Fatalf("deleted subrange mapping: %+v %v", mapped, err)
	}
	if mapped[0].Start != 2 || mapped[0].End != 3 {
		t.Fatal("original anchor mutated")
	}
	f.hook("PreToolUse", "rename")
	if err = os.Rename(filepath.Join(f.dir, "file.txt"), filepath.Join(f.dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	f.hook("PostToolUse", "rename")
	events, _ := f.s.Events(f.w.ID)
	last := events[len(events)-1]
	if _, err = f.s.CreateBatch(f.ctx, f.w.ID, "rename", "rename file", []string{last.ID}); err != nil {
		t.Fatal(err)
	}
	f.commit()
	r = f.refresh(r.ID)
	mapped, err = f.service.MappedThreads(f.ctx, r, r.Items[0].ID, r.Items[len(r.Items)-1].ID)
	if err != nil || len(mapped) != 1 || mapped[0].MappedPath != base64.StdEncoding.EncodeToString([]byte("renamed.txt")) {
		t.Fatalf("rename mapping: %+v %v", mapped, err)
	}
}
func TestDAGRangeProjectsBothBranchesAndMergeOnlyChanges(t *testing.T) {
	f := setup(t)
	base := f.git("rev-parse", "main")
	f.edit("feature", "a.txt", "feature\n")
	f.commit()
	f.git("switch", "-c", "side", base)
	f.edit("side", "b.txt", "side\n")
	f.commit()
	f.git("switch", "feature")
	f.git("merge", "--no-ff", "--no-commit", "side")
	f.write("merge-only.txt", "resolution\n")
	f.git("add", ".")
	f.git("commit", "-m", "merge")
	p, err := f.service.Preview(f.ctx, "feature", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Commits) != 2 {
		t.Fatalf("non-merge set: %+v", p.Commits)
	}
	r := f.create()
	if len(r.Items) != 3 || r.Items[2].Kind != "integrity_unknown" {
		t.Fatalf("missing merge-only correction: %+v", r.Items)
	}
	before, after, err := review.Selection(r, r.Items[0].ID, r.Items[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.r.Tree(f.ctx, p.Head)
	if err != nil || !domain.Equal(after, target) {
		t.Fatal("DAG result differs from final tree", err)
	}
	diff, err := f.r.Diff(f.ctx, before, after)
	if err != nil || len(diff) != 3 {
		t.Fatal("DAG diff omitted branch or merge changes", err)
	}
}

func TestAmendProjectsIntentWithoutInheritingApproval(t *testing.T) {
	f := setup(t)
	f.edit("first", "file.txt", "zero\nfirst\n")
	old := f.commit()
	r := f.create()
	original, err := f.s.Event(f.w.ID, r.Items[0].Events[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.Decision(f.ctx, f.w.RepositoryID, r.ID, r.Items[0].ID, "approved", r.Version); err != nil {
		t.Fatal(err)
	}
	f.git("commit", "--amend", "-m", "new message")
	head := f.git("rev-parse", "HEAD")
	mapping := old + " " + head + "\n"
	if err = capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"amend"}, strings.NewReader(mapping)); err != nil {
		t.Fatal(err)
	}
	next := f.refresh(r.ID)
	if next.Head != head || len(next.Items) != 1 || next.Items[0].Title != "first" || next.Items[0].Decision != "pending" || next.Items[0].ID == r.Items[0].ID {
		t.Fatalf("amend projection: %+v", next)
	}
	event, err := f.s.Event(f.w.ID, next.Items[0].Events[0])
	if err != nil || len(event.Origins) != 1 || event.Origins[0] != original.ID || !event.Projected {
		t.Fatal("missing origin", err)
	}
	unchanged, err := f.s.Event(f.w.ID, original.ID)
	if err != nil || unchanged.BeforeID != original.BeforeID || unchanged.AfterID != original.AfterID || unchanged.Base != original.Base || unchanged.Commit != old {
		t.Fatal("raw history rewritten", err)
	}
	count, _ := f.s.Events(f.w.ID)
	if err = capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"amend"}, strings.NewReader(mapping)); err != nil {
		t.Fatal(err)
	}
	again, _ := f.s.Events(f.w.ID)
	if len(again) != len(count) {
		t.Fatal("duplicate rewrite duplicated events")
	}
	projectedBatch := next.Items[0].SourceBatch
	if projectedBatch == "" {
		t.Fatal("rewritten batch has no manageable identity")
	}
	if err = f.s.DeleteBatch(f.ctx, f.w.ID, projectedBatch); err != nil {
		t.Fatal(err)
	}
	ungrouped := f.refresh(r.ID)
	if len(ungrouped.Items) != 1 || ungrouped.Items[0].Kind != "unbatched_patch" || ungrouped.Items[0].ID == next.Items[0].ID {
		t.Fatal("projected batch deletion was silently ignored")
	}
}
func TestAmendInsideToolPreservesPartialCommitPair(t *testing.T) {
	f := setup(t)
	f.edit("first", "file.txt", "ten\n")
	old := f.commit()
	t.Setenv("CODEX_THREAD_ID", "session")
	t.Setenv("CODEX_TOOL_CALL_ID", "amend")
	f.hook("PreToolUse", "amend")
	f.write("file.txt", "twenty\n")
	f.git("add", "file.txt")
	f.write("file.txt", "thirty\n")
	f.git("commit", "--amend", "-m", "partial amend")
	head := f.git("rev-parse", "HEAD")
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"amend"}, strings.NewReader(old+" "+head+"\n")); err != nil {
		t.Fatal(err)
	}
	f.hook("PostToolUse", "amend")
	events, err := f.s.Events(f.w.ID)
	if err != nil {
		t.Fatal(err)
	}
	var rawIDs []string
	for _, e := range events {
		if !e.Projected && e.Kind == "tool_edit" && e.BatchID == "" {
			rawIDs = append(rawIDs, e.ID)
		}
	}
	if len(rawIDs) != 1 {
		t.Fatalf("amend edit lost actual tool attribution: %+v", events)
	}
	if _, err = f.s.CreateBatch(f.ctx, f.w.ID, "amend value", "actual tool edit", rawIDs); err != nil {
		t.Fatal(err)
	}
	r := f.create()
	if r.Items[len(r.Items)-1].Kind != "commit_virtual" {
		t.Fatalf("partial amend close absent: %+v", r.Items)
	}
	f.commit()
	next := f.refresh(r.ID)
	for _, item := range next.Items {
		if item.Kind == "commit_virtual" {
			t.Fatal("paired carry/close did not cancel")
		}
	}
	target, err := f.r.Tree(f.ctx, next.Head)
	if err != nil || !domain.Equal(next.Items[len(next.Items)-1].After, target) {
		t.Fatal("partial amend did not close target", err)
	}
}
func TestRebaseAndSquashPreserveIndependentIntent(t *testing.T) {
	f := setup(t)
	f.edit("first", "a.txt", "a\n")
	old1 := f.commit()
	f.edit("second", "b.txt", "b\n")
	old2 := f.commit()
	r := f.create()
	for _, item := range r.Items {
		if err := f.s.Decision(f.ctx, f.w.RepositoryID, r.ID, item.ID, "approved", r.Version); err != nil {
			t.Fatal(err)
		}
	}
	f.git("switch", "main")
	f.write("upstream.txt", "upstream\n")
	f.git("add", ".")
	f.git("commit", "-m", "upstream")
	f.git("switch", "feature")
	f.git("rebase", "main")
	ids := strings.Fields(f.git("rev-list", "--reverse", "main..feature"))
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	mapping := old1 + " " + ids[0] + "\n" + old2 + " " + ids[1] + "\n"
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"rebase"}, strings.NewReader(mapping)); err != nil {
		t.Fatal(err)
	}
	next := f.refresh(r.ID)
	if len(next.Items) != 2 || next.Items[0].Title != "first" || next.Items[1].Title != "second" || next.Status != "in_review" {
		t.Fatalf("rebase intent lost: %+v", next)
	}
	f.git("reset", "--soft", "main")
	f.git("commit", "-m", "squash")
	squash := f.git("rev-parse", "HEAD")
	mapping = ids[0] + " " + squash + "\n" + ids[1] + " " + squash + "\n"
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"rebase"}, strings.NewReader(mapping)); err != nil {
		t.Fatal(err)
	}
	next = f.refresh(r.ID)
	if len(next.Items) != 2 || next.Items[0].Title != "first" || next.Items[1].Title != "second" {
		t.Fatalf("squash collapsed source intent: %+v", next)
	}
}
func TestSplitProjectsOnlyVerifiedPartsAndConflictIsUnknown(t *testing.T) {
	f := setup(t)
	f.hook("PreToolUse", "files")
	f.write("a.txt", "a\n")
	f.write("b.txt", "b\n")
	f.hook("PostToolUse", "files")
	events, _ := f.s.Events(f.w.ID)
	if _, err := f.s.CreateBatch(f.ctx, f.w.ID, "files", "two files", []string{events[0].ID}); err != nil {
		t.Fatal(err)
	}
	old := f.commit()
	f.git("switch", "-c", "split", "main")
	f.write("a.txt", "a\n")
	f.git("add", ".")
	f.git("commit", "-m", "part a")
	a := f.git("rev-parse", "HEAD")
	f.write("b.txt", "b\n")
	f.git("add", ".")
	f.git("commit", "-m", "part b")
	b := f.git("rev-parse", "HEAD")
	f.git("branch", "-f", "feature", b)
	mapping := old + " " + a + "\n" + old + " " + b + "\n"
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"rebase"}, strings.NewReader(mapping)); err != nil {
		t.Fatal(err)
	}
	r := f.create()
	if len(r.Items) != 2 {
		t.Fatalf("split duplicated or omitted changes: %+v", r.Items)
	}
	for _, item := range r.Items {
		diff, err := f.r.Diff(f.ctx, item.Before, item.After)
		if err != nil || len(diff) != 1 {
			t.Fatal("split copied the whole old patch", err)
		}
	}
	// Rewrite to a conflicting result: unchanged provenance must not be invented.
	f.git("switch", "feature")
	f.git("reset", "--hard", "main")
	f.write("a.txt", "different\n")
	f.git("add", ".")
	f.git("commit", "-m", "conflicting replacement")
	conflict := f.git("rev-parse", "HEAD")
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"rebase"}, strings.NewReader(old+" "+conflict+"\n")); err != nil {
		t.Fatal(err)
	}
	r = f.refresh(r.ID)
	foundUnknown := false
	for _, item := range r.Items {
		if strings.Contains(item.Title, "未知") {
			foundUnknown = true
		}
	}
	if !foundUnknown {
		t.Fatal("unmatched rewritten contents falsely inherited intent")
	}
}

func TestRebasePreservesVerifiedInverseVirtualPairs(t *testing.T) {
	f := setup(t)
	f.hook("PreToolUse", "partial")
	f.write("file.txt", "twenty\n")
	f.git("add", "file.txt")
	f.write("file.txt", "thirty\n")
	f.hook("PostToolUse", "partial")
	events, _ := f.s.Events(f.w.ID)
	if _, err := f.s.CreateBatch(f.ctx, f.w.ID, "partial", "intent", []string{events[0].ID}); err != nil {
		t.Fatal(err)
	}
	f.git("commit", "-m", "partial")
	if err := capture.Git(f.ctx, f.s, f.dir, "post-commit"); err != nil {
		t.Fatal(err)
	}
	old1 := f.git("rev-parse", "HEAD")
	old2 := f.commit()
	f.git("switch", "main")
	f.write("upstream.txt", "upstream\n")
	f.git("add", ".")
	f.git("commit", "-m", "upstream")
	f.git("switch", "feature")
	f.git("rebase", "main")
	ids := strings.Fields(f.git("rev-list", "--reverse", "main..feature"))
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	if err := capture.GitNotification(f.ctx, f.s, f.dir, "post-rewrite", []string{"rebase"}, strings.NewReader(old1+" "+ids[0]+"\n"+old2+" "+ids[1]+"\n")); err != nil {
		t.Fatal(err)
	}
	r := f.create()
	if len(r.Items) != 1 || r.Items[0].Title != "partial" {
		t.Fatalf("verified close/carry did not cancel after rebase: %+v", r.Items)
	}
}

func TestMergeOnlyRangeStillReviewsFinalChanges(t *testing.T) {
	f := setup(t)
	f.git("switch", "main")
	f.write("target.txt", "target advance\n")
	f.git("add", ".")
	f.git("commit", "-m", "target")
	f.git("switch", "feature")
	f.git("merge", "--no-ff", "--no-commit", "main")
	f.write("file.txt", "merge-only edit\n")
	f.git("add", ".")
	f.git("commit", "-m", "merge with unique edit")
	p, err := f.service.Preview(f.ctx, "feature", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Commits) != 0 || len(p.Diff) == 0 {
		t.Fatal("unexpected range", p)
	}
	r, err := f.service.Create(f.ctx, p, "merge-only changes")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 1 || r.Items[0].Kind != "integrity_unknown" || r.Status != "in_review" {
		t.Fatal("merge-only delta was lost", r)
	}
}

func TestCommitRangeSurvivesDeletedTrackingBranches(t *testing.T) {
	for _, branch := range []string{"feature", "main"} {
		t.Run(branch, func(t *testing.T) {
			f := setup(t)
			f.edit("edit", "file.txt", "zero\nnew\n")
			f.commit()
			cr := f.create()
			if err := f.s.Decision(f.ctx, f.w.RepositoryID, cr.ID, cr.Items[0].ID, "approved", cr.Version); err != nil {
				t.Fatal(err)
			}
			thread := domain.Thread{First: cr.Items[0].ID, Last: cr.Items[0].ID, Path: base64.StdEncoding.EncodeToString([]byte("file.txt")), Side: "new", Start: 2, End: 2}
			if err := f.service.AddThread(f.ctx, cr, thread, "saved", "Reviewer"); err != nil {
				t.Fatal(err)
			}
			f.git("switch", "--detach")
			f.git("branch", "-D", branch)
			// A movable surviving branch must not update only half the saved range.
			if branch == "main" {
				f.git("branch", "-f", "feature", cr.TargetOID)
			}
			if _, err := f.service.Refresh(f.ctx, cr.ID); err == nil {
				t.Fatal("missing branch update succeeded")
			}
			saved, err := f.s.Review(f.w.RepositoryID, cr.ID)
			if err != nil || saved.SourceOID != cr.SourceOID || saved.TargetOID != cr.TargetOID || saved.Version != cr.Version || saved.Status != "approved" || saved.Items[0].ID != cr.Items[0].ID {
				t.Fatalf("published review changed: %+v %v", saved, err)
			}
			comments, err := f.service.MappedThreads(f.ctx, saved, saved.Items[0].ID, saved.Items[0].ID)
			if err != nil || len(comments) != 1 {
				t.Fatal("saved comments unavailable", err)
			}
			p, err := f.service.PreviewRange(f.ctx, review.CommitRange{SourceOID: cr.SourceOID, TargetOID: cr.TargetOID})
			if err != nil || p.Head != cr.Head || len(p.Commits) != 1 {
				t.Fatal("commit range depends on branch", err)
			}
			fixed, err := f.service.Create(f.ctx, p, "fixed commits")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.service.Refresh(f.ctx, fixed.ID); err != nil {
				t.Fatal("fixed CR refresh", err)
			}
			f.git("branch", branch, map[string]string{"feature": cr.SourceOID, "main": cr.TargetOID}[branch])
			if branch == "main" {
				f.git("branch", "-f", "feature", cr.SourceOID)
			}
			restored := f.refresh(cr.ID)
			if restored.Version != cr.Version || restored.Items[0].Decision != "approved" || restored.Refresh.Code != "ok" {
				t.Fatal("branch restoration changed identity", restored)
			}
		})
	}
}

func TestUnchangedRefreshSkipsProjectionIO(t *testing.T) {
	f := setup(t)
	f.edit("edit", "file.txt", "new\n")
	f.commit()
	cr := f.create()
	// The fast path must not call the base reader, restore item snapshots, or build.
	statsBefore := f.r.ReadStats()
	if _, err := f.service.Refresh(f.ctx, cr.ID); err != nil {
		t.Fatal("unchanged refresh performed IO", err)
	}
	if f.r.ReadStats().TreeLoads != statsBefore.TreeLoads {
		t.Fatal("unchanged refresh rebuilt tree")
	}
	saved, err := f.s.ReviewSummary(f.w.RepositoryID, cr.ID)
	if err != nil || saved.Version != cr.Version || saved.Refresh.Code != "ok" {
		t.Fatal("unchanged input was rebuilt", saved.Refresh, err)
	}
}

func BenchmarkReviewRefresh(b *testing.B) {
	for _, count := range []int{1, 10, 50} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			f := setup(b)
			batches := []domain.Batch{}
			for i := 0; i < count; i++ {
				batches = append(batches, f.edit(fmt.Sprintf("batch-%d", i), fmt.Sprintf("file-%d", i), "new\n"))
			}
			f.commit()
			cr := f.create()
			b.Run("unchanged", func(b *testing.B) {
				before := f.r.ReadStats()
				_, work := f.s.TransactionStats()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := f.service.Refresh(f.ctx, cr.ID); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(f.r.ReadStats().TreeLoads-before.TreeLoads)/float64(b.N), "trees/op")
				_, afterWork := f.s.TransactionStats()
				b.ReportMetric(float64(afterWork-work)/float64(b.N), "txbody-ns/op")
			})
			b.Run("rebatch", func(b *testing.B) {
				batch := batches[0]
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := f.s.DeleteBatch(f.ctx, f.w.ID, batch.ID); err != nil {
						b.Fatal(err)
					}
					var err error
					batch, err = f.s.CreateBatch(f.ctx, f.w.ID, batch.Title, batch.Description, batch.Members)
					if err != nil {
						b.Fatal(err)
					}
					if _, err = f.service.Refresh(f.ctx, cr.ID); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
