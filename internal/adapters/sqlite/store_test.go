package store

import (
	"context"

	"errors"

	"strings"
	"testing"
	"time"

	"github.com/yhlooo/motif/internal/domain"
	"gorm.io/gorm"
)

func TestGORMConstraintsAndTransactionRollback(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	err = s.update(ctx, func(tx *gorm.DB) error {
		return tx.Create(&turnRecord{ID: domain.ID(), WorktreeID: "missing", Session: "s", ExternalID: "t"}).Error
	})
	if err == nil {
		t.Fatal("foreign key enforcement is disabled")
	}
	w, err := s.Register(ctx, "/repo.git", "/repo", "/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	err = s.update(ctx, func(tx *gorm.DB) error {
		first := turnRecord{ID: domain.ID(), WorktreeID: w.ID, Session: "s", ExternalID: "t"}
		if err := tx.Create(&first).Error; err != nil {
			return err
		}
		return tx.Create(&turnRecord{ID: domain.ID(), WorktreeID: w.ID, Session: "s", ExternalID: "t"}).Error
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
	init := domain.Observation{Worktree: w, Base: "base", BaseState: domain.From(map[string]domain.File{}), State: domain.From(map[string]domain.File{}), Trigger: "Stop"}
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
	o := domain.Observation{Worktree: w, Base: "base", BaseState: domain.From(map[string]domain.File{}), State: domain.From(map[string]domain.File{"file.txt": {Mode: "100644", ContentRef: hash}}), Trigger: "Stop", ExpectedVersion: cursor.Version}
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
	hash, _ := s.PutBlob([]byte("new"))
	if err = s.Observe(ctx, domain.Observation{Worktree: w, Base: "base", BaseState: domain.State{}, State: domain.From(map[string]domain.File{"new": {Mode: "100644", ContentRef: hash}}), Trigger: "Stop"}); err != nil {
		t.Fatal(err)
	}
	r := domain.Review{ID: domain.ID(), RepositoryID: w.RepositoryID, Title: "stale", Source: "feature", Target: "main", CheckSource: true, SourceRevision: revision}
	if err = s.SaveReview(ctx, w, &r); !errors.Is(err, ErrStaleView) {
		t.Fatal("mixed source revision was published", err)
	}
	list, err := s.Reviews(w.RepositoryID)
	if err != nil || len(list) != 0 {
		t.Fatal("failed publish left partial review", err)
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
	cr := domain.Review{ID: domain.ID(), Items: []domain.ReviewItem{{ID: domain.ID(), StableKey: "one", Before: domain.From(map[string]domain.File{}), After: domain.From(map[string]domain.File{})}, {ID: domain.ID(), StableKey: "two", Before: domain.From(map[string]domain.File{}), After: domain.From(map[string]domain.File{})}}}
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
