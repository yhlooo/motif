package store

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestLegacyV1DatabaseCompatibility(t *testing.T) {
	// This fixture was produced by the unchanged pre-GORM binary, not AutoMigrate.
	dir := t.TempDir()
	file, err := os.Open("testdata/v1.db.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "data.db"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	var ids struct {
		Repository string `json:"repository_id"`
		Worktree   string `json:"worktree_id"`
		Patch      string `json:"patch_id"`
		Batch      string `json:"batch_id"`
		Review     string `json:"review_id"`
		Item       string `json:"item_id"`
		Thread     string `json:"thread_id"`
	}
	raw, err = os.ReadFile("testdata/v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ids); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, content := range []string{"before\n", "after\n"} {
		if _, err = s.PutBlob([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	event, err := s.Event(ids.Worktree, ids.Patch)
	if err != nil {
		t.Fatal(err)
	}
	if event.BatchID != ids.Batch || event.BatchTitle != "legacy title" || event.BatchDescription != "legacy organized reason" || event.Call == nil || event.Call.Reason != "legacy original reason" || event.Commit == "" {
		t.Fatalf("legacy provenance changed: %+v", event)
	}
	r, err := s.Review(ids.Repository, ids.Review)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "approved" || len(r.Items) != 1 || r.Items[0].ID != ids.Item || r.SourceOID == "" || r.TargetOID == "" {
		t.Fatalf("legacy review changed: %+v", r)
	}
	threads, err := s.Threads(ids.Repository, ids.Review)
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].ID != ids.Thread || len(threads[0].Comments) != 2 || threads[0].Comments[0].Body != "legacy comment" || threads[0].Comments[1].Body != "legacy reply" {
		t.Fatalf("legacy comments changed: %+v", threads)
	}
	ctx := context.Background()
	if err = s.DeleteBatch(ctx, ids.Worktree, ids.Batch); err != nil {
		t.Fatal(err)
	}
	frozen, err := s.Review(ids.Repository, ids.Review)
	if err != nil || frozen.Items[0].Title != "legacy title" {
		t.Fatal("source deletion changed persisted legacy view", err)
	}
	if err = s.Reply(ctx, ids.Repository, ids.Thread, "new ORM reply", "Reviewer"); err != nil {
		t.Fatal(err)
	}
	if err = s.Decision(ctx, ids.Repository, ids.Review, ids.Item, "rejected"); err != nil {
		t.Fatal(err)
	}
	r, err = s.Review(ids.Repository, ids.Review)
	if err != nil || r.Status != "changes_requested" {
		t.Fatal("cannot update legacy review", err)
	}
	if err = s.DeleteRepository(ctx, ids.Repository); err != nil {
		t.Fatal("cannot cascade legacy project deletion", err)
	}
}

func TestGORMConstraintsAndTransactionRollback(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	err = s.update(ctx, func(tx *gorm.DB) error {
		return tx.Create(&turnRecord{ID: ID(), WorktreeID: "missing", Session: "s", ExternalID: "t"}).Error
	})
	if err == nil {
		t.Fatal("foreign key enforcement is disabled")
	}
	w, err := s.Register(ctx, "/repo.git", "/repo", "/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	err = s.update(ctx, func(tx *gorm.DB) error {
		first := turnRecord{ID: ID(), WorktreeID: w.ID, Session: "s", ExternalID: "t"}
		if err := tx.Create(&first).Error; err != nil {
			return err
		}
		return tx.Create(&turnRecord{ID: ID(), WorktreeID: w.ID, Session: "s", ExternalID: "t"}).Error
	})
	if err == nil {
		t.Fatal("duplicate turn identity accepted")
	}
	var count int64
	if err = s.db.Model(&turnRecord{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed transaction left partial data: count=%d err=%v", count, err)
	}
}

func TestConcurrentObservationsRejectStaleCursor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dir := t.TempDir()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	w, err := a.Register(ctx, "/repo.git", "/repo", "/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	init := Observation{Worktree: w, Base: "base", BaseState: State{}, State: State{}, Trigger: "Stop"}
	if err = a.Observe(ctx, init); err != nil {
		t.Fatal(err)
	}
	cursor, err := a.Cursor(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := a.PutBlob([]byte("changed\n"))
	if err != nil {
		t.Fatal(err)
	}
	o := Observation{Worktree: w, Base: "base", BaseState: State{}, State: State{"file.txt": {Mode: "100644", Hash: hash}}, Trigger: "Stop", ExpectedVersion: cursor.Version}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, s := range []*Store{a, b} {
		go func(s *Store) { <-start; results <- s.Observe(ctx, o) }(s)
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if strings.Contains(err.Error(), "capture cursor changed") {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one commit and one stale cursor, got %d/%d", successes, conflicts)
	}
	events, err := a.Events(w.ID)
	if err != nil || len(events) != 1 {
		t.Fatal("observation duplicated or lost", err)
	}
}

func TestSparseSnapshotsKeepOnlyOverridesAndRequireBase(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	w, err := s.Register(ctx, "/repo.git", "/repo", "/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("from Git\n")
	hash := s.RegisterGitBlob(content, "git-object", func() ([]byte, error) { return content, nil })
	base := State{"same.txt": {Mode: "100644", Hash: hash}, "deleted.txt": {Mode: "100644", Hash: hash}}
	s.SetBaseReader("/repo.git", func(context.Context, string) (State, error) { return Clone(base), nil })
	if err = s.Observe(ctx, Observation{Worktree: w, Base: "base", BaseState: base, State: base, Trigger: "Stop"}); err != nil {
		t.Fatal(err)
	}
	c, err := s.Cursor(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err = s.db.Model(&snapshotEntryRecord{}).Where("snapshot_id = ?", c.SnapshotID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("unchanged paths were duplicated", count, err)
	}
	changedHash, err := s.PutBlob([]byte("changed\n"))
	if err != nil {
		t.Fatal(err)
	}
	state := State{"same.txt": {Mode: "100755", Hash: hash}, "new.txt": {Mode: "100644", Hash: changedHash}}
	if err = s.Observe(ctx, Observation{Worktree: w, Base: "base", BaseState: base, State: state, Trigger: "Stop", ExpectedVersion: c.Version}); err != nil {
		t.Fatal(err)
	}
	c, err = s.Cursor(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	sn, err := s.Snapshot(c.SnapshotID)
	if err != nil || !Equal(sn.State, state) {
		t.Fatal("sparse state not restored", err)
	}
	var entries []snapshotEntryRecord
	if err = s.db.Where("snapshot_id = ?", c.SnapshotID).Find(&entries).Error; err != nil || len(entries) != 3 {
		t.Fatal("mode/add/delete overrides lost", err)
	}
	deleted := false
	for _, entry := range entries {
		deleted = deleted || entry.Deleted && string(entry.Path) == "deleted.txt"
	}
	if !deleted {
		t.Fatal("delete confused with empty content")
	}
	path, _ := s.blobPath(hash)
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unchanged Git content was duplicated on disk", err)
	}
	s.SetBaseReader("/repo.git", func(context.Context, string) (State, error) { return nil, fmt.Errorf("missing Git object") })
	if _, err = s.Snapshot(c.SnapshotID); err == nil {
		t.Fatal("missing baseline was silently reconstructed")
	}
}

func TestReviewPublishRejectsChangedSourceRevision(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	w, err := s.Register(ctx, "/repo.git", "/repo", "/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, revision, err := s.ReviewInputs(ctx, w.RepositoryID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Observe(ctx, Observation{Worktree: w, Base: "base", BaseState: State{}, State: State{}, Trigger: "Stop"}); err != nil {
		t.Fatal(err)
	}
	r := Review{ID: ID(), RepositoryID: w.RepositoryID, Title: "stale", Source: "feature", Target: "main", CheckSource: true, SourceRevision: revision}
	if err = s.SaveReview(ctx, w, &r); !errors.Is(err, ErrStaleView) {
		t.Fatal("mixed source revision was published", err)
	}
	list, err := s.Reviews(w.RepositoryID)
	if err != nil || len(list) != 0 {
		t.Fatal("failed publish left partial review", err)
	}
}

func TestLegacyReviewRefreshOnlyAdoptsProvableIdentity(t *testing.T) {
	for _, kind := range []string{"integrity_unknown", "batch"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s, err := Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			w, err := s.Register(ctx, "/legacy.git", "/legacy", "/legacy.git")
			if err != nil {
				t.Fatal(err)
			}
			hash, err := s.PutBlob([]byte("legacy content\n"))
			if err != nil {
				t.Fatal(err)
			}
			itemID := ID()
			r := Review{ID: ID(), RepositoryID: w.RepositoryID, Title: "legacy", Base: "base", Head: "head", Items: []ReviewItem{{ID: itemID, Kind: kind, Title: "same title", Before: State{}, After: State{"file": {Mode: "100644", Hash: hash}}}}}
			if err = s.SaveReview(ctx, w, &r); err != nil {
				t.Fatal(err)
			}
			if err = s.Decision(ctx, w.RepositoryID, r.ID, itemID, "approved"); err != nil {
				t.Fatal(err)
			}
			r.Items[0].ID = ID()
			r.Items[0].StableKey = "head/" + kind + "/source"
			r.Items[0].Commit = "head"
			r.ViewDigest = "new view"
			if kind == "batch" {
				r.Items[0].SourceBatch = "new batch identity"
			}
			if err = s.PublishReview(ctx, w, &r, 1); err != nil {
				t.Fatal(err)
			}
			current, err := s.Review(w.RepositoryID, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "integrity_unknown" {
				if current.Items[0].ID != itemID || current.Status != "approved" || current.Items[0].StableKey == "" {
					t.Fatal("provable identity lost", current)
				}
			} else {
				if current.Items[0].ID == itemID || current.Items[0].Decision != "pending" {
					t.Fatal("unproven batch inherited approval", current)
				}
				var historical reviewItemRecord
				if err = s.db.Where("id = ?", itemID).Take(&historical).Error; err != nil || historical.Active || historical.Decision != "approved" {
					t.Fatal("old approval was not preserved", historical, err)
				}
			}
		})
	}
}

// A base reader performs a second database read. With the single-connection pool
// this times out if snapshot resolution accidentally runs inside a transaction.
func TestSnapshotResolutionRunsOutsideTransactions(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	w, err := s.Register(ctx, "/prepared.git", "/prepared", "/prepared.git")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := s.PutBlob([]byte("content\n"))
	if err != nil {
		t.Fatal(err)
	}
	state := State{"file": {Mode: "100644", Hash: hash}}
	calls := 0
	s.SetBaseReader("/prepared.git", func(ctx context.Context, oid string) (State, error) {
		calls++
		limited, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		if _, err := s.SourceRevision(limited); err != nil {
			return nil, fmt.Errorf("base IO held database connection: %w", err)
		}
		if oid == "new" {
			return state, nil
		}
		return State{}, nil
	})
	o := Observation{Worktree: w, Base: "base", BaseState: State{}, State: state, Trigger: "Stop"}
	if err = s.Observe(ctx, o); err != nil {
		t.Fatal(err)
	}
	cr := Review{ID: ID(), Base: "base", Head: "new", Items: []ReviewItem{{ID: ID(), Kind: "integrity_unknown", Before: State{}, After: state}}}
	if err = s.SaveReview(ctx, w, &cr); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Review(w.RepositoryID, cr.ID); err != nil {
		t.Fatal(err)
	}
	c, err := s.Cursor(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	resume := Observation{Worktree: w, Base: "new", BaseState: state, State: state, Trigger: "rewrite_resume", Reset: true, ExpectedVersion: c.Version}
	if err = s.SaveRewrite(ctx, w, "rebase", []RewriteMapping{{Old: "old", New: "new"}}, []CommitProjection{{Parent: "base", Commit: "new", Changes: []ProjectedChange{{Kind: "unknown", Before: State{}, After: state}}}}, nil, c.Version, resume); err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("base reader was not exercised")
	}
}

func TestRefreshAttemptsAndAtomicDecisions(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	w, err := s.Register(ctx, "/attempt.git", "/attempt", "/attempt.git")
	if err != nil {
		t.Fatal(err)
	}
	cr := Review{ID: ID(), Items: []ReviewItem{{ID: ID(), Before: State{}, After: State{}}, {ID: ID(), Before: State{}, After: State{}}}}
	if err = s.SaveReview(ctx, w, &cr); err != nil {
		t.Fatal(err)
	}
	first, err := s.BeginRefresh(ctx, w.RepositoryID, cr.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.BeginRefresh(ctx, w.RepositoryID, cr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishRefresh(ctx, w.RepositoryID, cr.ID, second, "ok", ""); err != nil {
		t.Fatal(err)
	}
	if err = s.FinishRefresh(ctx, w.RepositoryID, cr.ID, first, "build_failed", "late failure"); err != nil {
		t.Fatal(err)
	}
	if err = s.Decisions(ctx, w.RepositoryID, cr.ID, []string{cr.Items[0].ID, "missing"}, "approved", cr.Version); err == nil {
		t.Fatal("invalid selection accepted")
	}
	saved, err := s.ReviewSummary(w.RepositoryID, cr.ID)
	if err != nil || saved.Refresh.Code != "ok" || saved.Items[0].Decision != "pending" {
		t.Fatal("late attempt or partial decision corrupted state", saved, err)
	}
}
