package sqlite

import (
	"context"
	"errors"
	"fmt"
	"github.com/yhlooo/motif/internal/domain"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) (*Store, domain.Worktree) {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	w, e := s.Register(context.Background(), "/repo.git", "/repo", "/repo.git")
	if e != nil {
		t.Fatal(e)
	}
	return s, w
}
func testState(t *testing.T, s *Store, value string) domain.State {
	t.Helper()
	ref, e := s.PutBlob([]byte(value))
	if e != nil {
		t.Fatal(e)
	}
	return domain.From(map[string]domain.File{"file": {Mode: "100644", ContentRef: ref}})
}
func TestImmutableProjectionPublication(t *testing.T) {
	s, w := testStore(t)
	ctx := context.Background()
	firstState := testState(t, s, "one")
	secondState := testState(t, s, "two")
	r := domain.Review{ID: domain.ID(), ViewDigest: "first", Items: []domain.ReviewItem{{StableKey: "commit/batch", Before: domain.State{}, After: firstState}}}
	if e := s.SaveReview(ctx, w, &r); e != nil {
		t.Fatal(e)
	}
	firstProjection := r.ProjectionID
	if e := s.Decision(ctx, w.RepositoryID, r.ID, r.Items[0].ID, "approved", r.Version); e != nil {
		t.Fatal(e)
	}
	next := r
	next.ViewDigest = "second"
	next.Items = []domain.ReviewItem{{StableKey: "commit/batch", Before: domain.State{}, After: secondState}}
	if e := s.PublishReview(ctx, w, &next, r.Version); e != nil {
		t.Fatal(e)
	}
	if next.ProjectionID == firstProjection {
		t.Fatal("projection overwritten")
	}
	old, e := s.PublishedReview(w.RepositoryID, r.ID, firstProjection)
	if e != nil {
		t.Fatal(e)
	}
	old, e = s.LoadReviewStates(old)
	if e != nil || !domain.Equal(old.Items[0].After, firstState) {
		t.Fatal("old projection changed", e)
	}
	current, e := s.Review(w.RepositoryID, r.ID)
	if e != nil || !domain.Equal(current.Items[0].After, secondState) || current.Items[0].Decision != "approved" {
		t.Fatal("identity or content lost", e)
	}
	if e = s.Decision(ctx, w.RepositoryID, r.ID, r.Items[0].ID, "rejected", r.Version); !errors.Is(e, ErrStaleView) {
		t.Fatal("old projection wrote decision", e)
	}
	var rows []eventRecord
	if e = s.db.Find(&rows).Error; e != nil || len(rows) != 0 {
		t.Fatal("review wrote source history", e)
	}
	var projections int64
	s.db.Model(&reviewProjectionRecord{}).Count(&projections)
	if projections != 2 {
		t.Fatal("missing immutable history")
	}
}
func TestSourceRevisionTracksRealChangesPerRepository(t *testing.T) {
	s, w := testStore(t)
	ctx := context.Background()
	other, e := s.Register(ctx, "/other.git", "/other", "/other.git")
	if e != nil {
		t.Fatal(e)
	}
	o := domain.Observation{Worktree: w, Base: "base", Trigger: "Stop", State: testState(t, s, "one"), Receipt: "receipt", Digest: "one"}
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	v, _ := s.SourceRevision(ctx, w.RepositoryID)
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	same, _ := s.SourceRevision(ctx, w.RepositoryID)
	if same != v {
		t.Fatal("duplicate invalidated source")
	}
	c, _ := s.Cursor(w.ID)
	o.Receipt = ""
	o.ExpectedVersion = c.Version
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	same, _ = s.SourceRevision(ctx, w.RepositoryID)
	if same != v {
		t.Fatal("unchanged observation invalidated source")
	}
	if e = s.Observe(ctx, domain.Observation{Worktree: other, Base: "base", Trigger: "Stop", State: testState(t, s, "other")}); e != nil {
		t.Fatal(e)
	}
	same, _ = s.SourceRevision(ctx, w.RepositoryID)
	if same != v {
		t.Fatal("other repository invalidated source")
	}
	o.Receipt = "receipt"
	o.Digest = "conflict"
	if e = s.Observe(ctx, o); e == nil {
		t.Fatal("receipt conflict accepted")
	}
}
func TestSelectedDiffDoesNotResolveUnselectedSnapshots(t *testing.T) {
	s, w := testStore(t)
	a := testState(t, s, "a")
	b := testState(t, s, "b")
	r := domain.Review{ID: domain.ID(), ViewDigest: "selected", Items: []domain.ReviewItem{{StableKey: "a", Before: domain.State{}, After: a}, {StableKey: "b", Before: a, After: b}}}
	if e := s.SaveReview(context.Background(), w, &r); e != nil {
		t.Fatal(e)
	}
	loaded, e := s.ReviewSummary(w.RepositoryID, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	// Break only the unselected final state object and discard the disposable cache.
	path, _ := s.blobPath(b.Root())
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	s.nodes = map[string]domain.State{}
	selected, e := s.LoadSelection(loaded, loaded.Items[0].ID, loaded.Items[0].ID, false)
	if e != nil || !domain.Equal(selected.Items[0].After, a) {
		t.Fatal("selected read touched unrelated snapshot", e)
	}
	if _, e = s.LoadReviewStates(loaded); e == nil {
		t.Fatal("missing selected content not diagnosed")
	}
}
func TestLifecycleLeaseProtectsPublicationFromUninstall(t *testing.T) {
	s, w := testStore(t)
	ctx := context.Background()
	state := testState(t, s, "shared")
	if e := s.Observe(ctx, domain.Observation{Worktree: w, Trigger: "Stop", State: state}); e != nil {
		t.Fatal(e)
	}
	release, e := s.lease(false)
	if e != nil {
		t.Fatal(e)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() { close(started); done <- s.DeleteRepository(ctx, w.RepositoryID) }()
	<-started
	select {
	case e := <-done:
		release()
		t.Fatal("exclusive delete crossed active lease", e)
	case <-time.After(30 * time.Millisecond):
	}
	release()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("delete did not finish")
	}
	ref := state.Get("file").ContentRef
	path, _ := s.blobPath(ref)
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("exclusive content retained", e)
	}
}
func TestStateObjectsReopenAndAreProjectScoped(t *testing.T) {
	s, w := testStore(t)
	ctx := context.Background()
	state := testState(t, s, "same")
	o := domain.Observation{Worktree: w, Trigger: "Stop", State: state}
	if e := s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	other, e := s.Register(ctx, "/second.git", "/second", "/second.git")
	if e != nil {
		t.Fatal(e)
	}
	o.Worktree = other
	if e = s.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteRepository(ctx, w.RepositoryID); e != nil {
		t.Fatal(e)
	}
	c, _ := s.Cursor(other.ID)
	s.nodes = map[string]domain.State{}
	snapshot, e := s.Snapshot(c.SnapshotID)
	if e != nil || !domain.Equal(snapshot.State, state) {
		t.Fatal("shared nodes deleted", e)
	}
	if _, e = s.Blob(state.Get("file").ContentRef); e != nil {
		t.Fatal("shared bytes deleted", e)
	}
}
func TestOlderSchemaIsRejectedWithoutMigration(t *testing.T) {
	s, _ := testStore(t)
	if e := s.db.Model(&schemaMigration{}).Where("version = ?", schemaVersion).Update("version", 7).Error; e != nil {
		t.Fatal(e)
	}
	if _, e := Open(s.Dir); e == nil {
		t.Fatal("old database silently migrated")
	}
	var v schemaMigration
	if e := s.db.Take(&v).Error; e != nil || v.Version != 7 {
		t.Fatal(fmt.Sprint(v), e)
	}
}

func TestConcurrentDecisionRevisionRejectsLostUpdate(t *testing.T) {
	s, w := testStore(t)
	ctx := context.Background()
	r := domain.Review{ID: domain.ID(), ViewDigest: "decision", Items: []domain.ReviewItem{{StableKey: "a"}, {StableKey: "b"}}}
	if e := s.SaveReview(ctx, w, &r); e != nil {
		t.Fatal(e)
	}
	current, e := s.ReviewSummary(w.RepositoryID, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	ids := []string{current.Items[0].ID, current.Items[1].ID}
	expected := map[string]int64{}
	for _, item := range current.Items {
		expected[item.ID] = item.DecisionRevision
	}
	if e = s.Decisions(ctx, w.RepositoryID, r.ID, ids, "approved", r.Version, expected); e != nil {
		t.Fatal(e)
	}
	if e = s.Decisions(ctx, w.RepositoryID, r.ID, ids, "rejected", r.Version, expected); !errors.Is(e, ErrStaleView) {
		t.Fatal("stale decision overwrote a concurrent approval", e)
	}
	current, e = s.ReviewSummary(w.RepositoryID, r.ID)
	if e != nil || current.Status != "approved" {
		t.Fatal("partial stale decision", e)
	}
}

func TestNormalOpenDoesNotWaitForActiveCaptureLease(t *testing.T) {
	s, _ := testStore(t)
	release, e := s.CaptureLease(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		other, e := Open(s.Dir)
		if e == nil {
			e = other.Close()
		}
		done <- e
	}()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("normal open waited for shared capture lease")
	}
}
