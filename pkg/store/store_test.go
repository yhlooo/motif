package store

import (
	"compress/gzip"
	"context"
	"encoding/json"
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
		t.Fatal("source deletion changed frozen legacy review", err)
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
