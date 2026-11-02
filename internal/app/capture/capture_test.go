package capture_test

import (
	"context"
	"encoding/json"
	"github.com/yhlooo/motif/internal/app/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/app/capture"
	"github.com/yhlooo/motif/internal/app/review"
	"github.com/yhlooo/motif/internal/domain"
)

type fixture struct {
	t         *testing.T
	ctx       context.Context
	dir, data string
	s         *sqlite.Store
	r         *workspace.Project
	w         domain.Worktree
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, ctx: context.Background(), dir: t.TempDir(), data: t.TempDir()}
	f.git("init", "-b", "main")
	f.git("config", "user.name", "Test User")
	f.git("config", "user.email", "test@example.com")
	f.write("value.txt", "10\n")
	f.git("add", ".")
	f.git("commit", "-m", "base")
	f.git("switch", "-c", "feature")
	var err error
	f.s, err = sqlite.Open(f.data)
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
	return f
}
func (f *fixture) git(args ...string) string {
	f.t.Helper()
	c := exec.Command("git", args...)
	c.Dir = f.dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		f.t.Fatalf("git %v: %s: %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func (f *fixture) write(path, content string) {
	f.t.Helper()
	p := filepath.Join(f.dir, path)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) hook(event, id, turn string) map[string]string {
	f.t.Helper()
	in := capture.CodexInput{Event: event, CWD: f.dir, Session: "session", Turn: turn, ToolID: id, Tool: "Bash", Model: "test-model", Input: json.RawMessage(`{"justification":"原始工具说明"}`)}
	raw, _ := json.Marshal(in)
	out, err := capture.Codex(f.ctx, f.s, raw)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}
func (f *fixture) events() []domain.Event {
	f.t.Helper()
	es, e := f.s.Events(f.w.ID)
	if e != nil {
		f.t.Fatal(e)
	}
	return es
}
func (f *fixture) commit(message string) {
	f.t.Helper()
	f.git("commit", "-m", message)
	if e := capture.Git(f.ctx, f.s, f.dir, "post-commit"); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) cr(source string) domain.Review {
	f.t.Helper()
	service := review.Service{Repo: f.r, Worktree: f.w}
	p, e := service.Preview(f.ctx, source, "main")
	if e != nil {
		f.t.Fatal(e)
	}
	r, e := service.Create(f.ctx, p, "测试审查")
	if e != nil {
		f.t.Fatal(e)
	}
	return r
}

func TestCaptureBatchStopAndPersistedView(t *testing.T) {
	f := newFixture(t)
	f.write("dirty.txt", "external\n")
	f.hook("PreToolUse", "one", "turn")
	f.hook("PreToolUse", "one", "turn") // Idempotent retry.
	f.write("value.txt", "30\n")
	f.write("new.txt", "hello\n")
	f.hook("PostToolUse", "one", "turn")
	f.hook("PostToolUse", "one", "turn")
	es := f.events()
	if len(es) != 2 || es[0].Kind != "unknown" || es[1].Kind != "tool_edit" {
		t.Fatalf("events: %+v", es)
	}
	if es[1].Call.Reason != "原始工具说明" || es[0].Call != nil {
		t.Fatal("source attribution was lost")
	}
	for i := 0; i < 2; i++ {
		out := f.hook("Stop", "", "turn")
		if out["decision"] != "block" || !strings.Contains(out["reason"], es[1].ID) {
			t.Fatalf("stop: %v", out)
		}
	}
	turn, e := f.s.ResolveTurn(f.w.ID, "session", "one")
	if e != nil || turn != es[1].TurnID {
		t.Fatal("resolve turn", e)
	}
	if _, e = f.s.ResolveTurn(f.w.ID, "", ""); e == nil {
		t.Fatal("manual context must not guess a turn")
	}
	batch, e := f.s.CreateBatch(f.ctx, f.w.ID, "Expose value", "整理后的原因", []string{es[1].ID})
	if e != nil {
		t.Fatal(e)
	}
	same, e := f.s.CreateBatch(f.ctx, f.w.ID, batch.Title, batch.Description, []string{es[1].ID})
	if e != nil || same.ID != batch.ID {
		t.Fatal("batch retry", e)
	}
	if out := f.hook("Stop", "", "turn"); out != nil {
		t.Fatal(out)
	}
	f.git("add", ".")
	f.commit("feature")
	cr := f.cr("feature")
	if cr.Integrity != "verified" || len(cr.Items) != 2 {
		t.Fatalf("review %+v", cr)
	}
	if e = f.s.DeleteBatch(f.ctx, f.w.ID, batch.ID); e != nil {
		t.Fatal(e)
	}
	frozen, e := f.s.Review(f.w.RepositoryID, cr.ID)
	if e != nil || frozen.Items[1].Title != batch.Title {
		t.Fatal("persisted view changed before refresh", e)
	}
	if out := f.hook("Stop", "", "turn"); out["decision"] != "block" {
		t.Fatal("deleted batch must need regrouping")
	}
	for _, item := range frozen.Items {
		if e = f.s.Decision(f.ctx, f.w.RepositoryID, cr.ID, item.ID, "approved", cr.Version); e != nil {
			t.Fatal(e)
		}
	}
	frozen, e = f.s.Review(f.w.RepositoryID, cr.ID)
	if e != nil || frozen.Status != "approved" {
		t.Fatal("approval aggregation", e)
	}
	if e = f.s.Decision(f.ctx, f.w.RepositoryID, cr.ID, frozen.Items[0].ID, "rejected", cr.Version); e != nil {
		t.Fatal(e)
	}
	frozen, _ = f.s.Review(f.w.RepositoryID, cr.ID)
	if frozen.Status != "changes_requested" {
		t.Fatal("rejection aggregation")
	}
	// Reopen to verify actual SQLite persistence, not just in-process state.
	if e = f.s.Close(); e != nil {
		t.Fatal(e)
	}
	f.s, e = sqlite.Open(f.data)
	if e != nil {
		t.Fatal(e)
	}
	f.r, e = workspace.Open(f.ctx, f.dir, f.s)
	if e != nil {
		t.Fatal(e)
	}
	reopened, e := f.s.Review(f.w.RepositoryID, cr.ID)
	if e != nil || reopened.Items[1].Title != batch.Title {
		t.Fatal("persistence", e)
	}
}
func TestPartialCommitVirtualPairAndCombinedNetDiff(t *testing.T) {
	f := newFixture(t)
	f.hook("PreToolUse", "one", "turn")
	f.write("value.txt", "20\n")
	f.git("add", "value.txt")
	f.write("value.txt", "30\n")
	f.hook("PostToolUse", "one", "turn")
	ai := f.events()[0]
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "Change value", "intent", []string{ai.ID}); e != nil {
		t.Fatal(e)
	}
	f.commit("partial")
	firstCommit := f.git("rev-parse", "HEAD")
	f.git("branch", "partial")
	es := f.events()
	if len(es) != 3 || es[1].Kind != "virtual_close" || es[2].Kind != "virtual_carry" || es[1].PairID != es[2].PairID {
		t.Fatalf("virtual events %+v", es)
	}
	if es[0].Base == es[0].Commit || es[0].Commit != firstCommit {
		t.Fatal("original baseline overwritten")
	}
	cr := f.cr("partial")
	if len(cr.Items) != 2 || cr.Items[1].Kind != "commit_virtual" {
		t.Fatalf("partial CR %+v", cr.Items)
	}
	f.hook("PreToolUse", "two", "turn")
	f.write("value.txt", "40\n")
	f.hook("PostToolUse", "two", "turn")
	es = f.events()
	last := es[len(es)-1]
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "Update value", "followup", []string{last.ID}); e != nil {
		t.Fatal(e)
	}
	f.git("add", ".")
	f.commit("rest")
	cr = f.cr("feature")
	if len(cr.Items) != 2 {
		t.Fatalf("inverse virtual pair should cancel: %+v", cr.Items)
	}
	b, a, e := review.Selection(cr, cr.Items[0].ID, cr.Items[1].ID)
	if e != nil {
		t.Fatal(e)
	}
	diff, e := f.r.Diff(f.ctx, b, a)
	if e != nil || len(diff) != 1 || !strings.Contains(diff[0].Patch, "-10\n+40") {
		t.Fatalf("net diff: %+v %v", diff, e)
	}
}
func TestToolCommitBoundaryAndMissingPost(t *testing.T) {
	f := newFixture(t)
	t.Setenv("CODEX_THREAD_ID", "session")
	t.Setenv("CODEX_TOOL_CALL_ID", "one")
	f.hook("PreToolUse", "one", "turn")
	f.write("value.txt", "20\n")
	f.git("add", ".")
	f.commit("inside tool")
	f.write("value.txt", "30\n")
	f.hook("PostToolUse", "one", "turn")
	es := f.events()
	if len(es) != 2 || es[0].Kind != "tool_edit" || es[1].Kind != "tool_edit" || es[0].CallID != es[1].CallID {
		t.Fatalf("nested commit events %+v", es)
	}
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "Invalid", "cross commit", []string{es[0].ID, es[1].ID}); e == nil {
		t.Fatal("cross-boundary batch accepted")
	}
	f.hook("PreToolUse", "lost", "turn")
	f.write("value.txt", "50\n")
	f.hook("PreToolUse", "next", "turn")
	es = f.events()
	if es[len(es)-1].Kind != "unknown" || es[len(es)-1].Call != nil {
		t.Fatal("missing post was falsely attributed")
	}
	count := len(es)
	f.hook("PostToolUse", "lost", "turn")
	if len(f.events()) != count {
		t.Fatal("late post duplicated history")
	}
	f.hook("PostToolUse", "next", "turn")
}
func TestUnknownInterruptsBatchAndRollback(t *testing.T) {
	f := newFixture(t)
	f.hook("PreToolUse", "one", "turn")
	f.write("value.txt", "20\n")
	f.hook("PostToolUse", "one", "turn")
	f.write("external.txt", "external\n")
	f.hook("PreToolUse", "two", "turn")
	f.write("value.txt", "30\n")
	f.hook("PostToolUse", "two", "turn")
	es := f.events()
	if len(es) != 3 {
		t.Fatalf("events %v", es)
	}
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "invalid", "skipped unknown", []string{es[0].ID, es[2].ID}); e == nil {
		t.Fatal("noncontiguous batch accepted")
	}
	for _, e := range f.events() {
		if e.Kind == "tool_edit" && e.BatchID != "" {
			t.Fatal("failed creation left partial members")
		}
	}
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "invalid", "mixed provenance", []string{es[0].ID, es[1].ID}); e == nil {
		t.Fatal("unknown accepted as AI")
	}
	if _, e := f.s.CreateBatch(f.ctx, f.w.ID, "invalid", "duplicate", []string{es[0].ID, es[0].ID}); e == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestUncapturedAndChangedPreview(t *testing.T) {
	f := newFixture(t)
	f.write("value.txt", "20\n")
	f.git("add", ".")
	f.git("commit", "-m", "no hook")
	service := review.Service{Repo: f.r, Worktree: f.w}
	p, e := service.Preview(f.ctx, "feature", "main")
	if e != nil {
		t.Fatal(e)
	}
	cr, e := service.Create(f.ctx, p, "unknown")
	if e != nil || len(cr.Items) != 1 || cr.Items[0].Kind != "integrity_unknown" || cr.Warning == "" {
		t.Fatalf("unknown CR: %+v %v", cr, e)
	}
	f.write("value.txt", "30\n")
	f.git("add", ".")
	f.git("commit", "-m", "later")
	if _, e = service.Create(f.ctx, p, "stale"); e == nil {
		t.Fatal("stale branch input was accepted")
	}
}

func TestProjectDeletionPreservesSharedBlobs(t *testing.T) {
	f := newFixture(t)
	f.hook("PreToolUse", "one", "turn")
	f.write("value.txt", "20\n")
	f.write("private.txt", "exclusive\n")
	f.hook("PostToolUse", "one", "turn")
	f.git("add", ".")
	f.commit("change")
	f.cr("feature")
	other := newFixture(t)
	other.r.Store = f.s
	w, e := other.r.Identity(f.ctx)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(capture.CodexInput{Event: "PreToolUse", CWD: other.dir, Session: "other", Turn: "turn", ToolID: "call", Tool: "Bash"})
	if _, e = capture.Codex(f.ctx, f.s, raw); e != nil {
		t.Fatal(e)
	}
	other.write("value.txt", "20\n")
	raw, _ = json.Marshal(capture.CodexInput{Event: "PostToolUse", CWD: other.dir, Session: "other", Turn: "turn", ToolID: "call", Tool: "Bash"})
	if _, e = capture.Codex(f.ctx, f.s, raw); e != nil {
		t.Fatal(e)
	}
	cursor, e := f.s.Cursor(w.ID)
	if e != nil {
		t.Fatal(e)
	}
	snap, e := f.s.Snapshot(cursor.SnapshotID)
	if e != nil {
		t.Fatal(e)
	}
	sharedHash := snap.State.Get("value.txt").ContentRef
	plan, e := f.s.DeletionPlan(f.w.RepositoryID)
	if e != nil || plan.SharedBlobs == 0 || plan.ExclusiveBlobs == 0 {
		t.Fatalf("deletion plan %+v %v", plan, e)
	}
	if e = f.s.DeleteRepository(f.ctx, f.w.RepositoryID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Snapshot(cursor.SnapshotID); e != nil {
		t.Fatal("deleted other project's snapshot", e)
	}
	b, e := f.s.Blob(sharedHash)
	if e != nil || string(b) != "20\n" {
		t.Fatal("deleted shared blob", e)
	}
}

func TestSameHeadCheckoutPreservesPendingHistory(t *testing.T) {
	f := newFixture(t)
	f.hook("PreToolUse", "one", "turn")
	f.write("value.txt", "20\n")
	f.hook("PostToolUse", "one", "turn")
	f.git("switch", "-c", "other")
	if e := capture.Git(f.ctx, f.s, f.dir, "post-checkout"); e != nil {
		t.Fatal(e)
	}
	f.git("add", ".")
	f.commit("after checkout")
	es := f.events()
	if len(es) != 1 || es[0].Kind != "tool_edit" || es[0].Commit == "" {
		t.Fatalf("same-head checkout abandoned history: %+v", es)
	}
}

func TestFailedObservationDoesNotAdvanceCursor(t *testing.T) {
	f := newFixture(t)
	f.hook("PreToolUse", "one", "turn")
	f.hook("PostToolUse", "one", "turn")
	before, e := f.s.Cursor(f.w.ID)
	if e != nil {
		t.Fatal(e)
	}
	sn, e := f.s.Snapshot(before.SnapshotID)
	if e != nil {
		t.Fatal(e)
	}
	o := domain.Observation{Worktree: f.w, Base: before.Base, State: sn.State, BaseState: sn.State, Trigger: "Stop", ExpectedVersion: before.Version - 1}
	if e = f.s.Observe(f.ctx, o); e == nil {
		t.Fatal("stale cursor was accepted")
	}
	after, e := f.s.Cursor(f.w.ID)
	if e != nil || after != before {
		t.Fatal("failed cursor validation advanced state", e)
	}
	o.ExpectedVersion = before.Version
	o.State = domain.From(map[string]domain.File{"missing": domain.File{Mode: "100644", ContentRef: strings.Repeat("a", 64)}})
	if e = f.s.Observe(f.ctx, o); e == nil {
		t.Fatal("missing blob accepted")
	}
	after, e = f.s.Cursor(f.w.ID)
	if e != nil || after != before {
		t.Fatal("missing content advanced state", e)
	}
}
