package sqlite

import (
	"context"

	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
	"github.com/yhlooo/motif/internal/domain/state"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var ErrNotFound = gorm.ErrRecordNotFound

type Store struct {
	transactionWork atomic.Int64
	transactions    atomic.Int64
	mu              sync.Mutex
	nodes           map[string]domain.State

	db  *gorm.DB
	Dir string
}

func Open(dir string) (*Store, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "data.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	// Configure every connection through the pure-Go driver's DSN. In particular,
	// immediate transactions acquire the write lock before reading a cursor.
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	opts := url.Values{"_txlock": {"immediate"}, "_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)"}}
	uri.RawQuery = opts.Encode()
	db, err := gorm.Open(sqlite.Open(uri.String()), &gorm.Config{SkipDefaultTransaction: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(1)
	s := &Store{db: db, Dir: dir, nodes: map[string]domain.State{}}
	release, lockErr := s.lease(true)
	if lockErr != nil {
		pool.Close()
		return nil, lockErr
	}
	defer release()
	if err = s.migrate(); err != nil {
		_ = pool.Close()
		return nil, err
	}
	if err = s.drainDeletes(); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

const schemaVersion = 8

func (s *Store) migrate() error {
	if s.db.Migrator().HasTable(&schemaMigration{}) {
		var v schemaMigration
		if err := s.db.Take(&v).Error; err != nil {
			return err
		}
		if v.Version != schemaVersion {
			return fmt.Errorf("database schema %d is incompatible with %d; choose a new MOTIF_DATA_DIR or explicitly remove development data", v.Version, schemaVersion)
		}
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(models()...); err != nil {
			return err
		}
		return tx.Create(&schemaMigration{Version: schemaVersion}).Error
	})
}
func (s *Store) Close() error {
	pool, err := s.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}
func (s *Store) update(ctx context.Context, fn func(*gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		start := time.Now()
		s.transactions.Add(1)
		defer func() { s.transactionWork.Add(time.Since(start).Nanoseconds()) }()
		return fn(tx)
	})
}
func (s *Store) sourceUpdate(ctx context.Context, fn func(*gorm.DB) error) error {
	return s.update(ctx, fn)
}
func bumpSource(tx *gorm.DB, w string) error {
	return tx.Model(&repositoryRecord{}).Where("id IN (?)", tx.Model(&worktreeRecord{}).Select("repository_id").Where("id = ?", w)).Update("generation", gorm.Expr("generation + 1")).Error
}
func associate(tx *gorm.DB, ids []string, commit string) error {
	for _, id := range ids {
		r := commitChangeRecord{EventID: id, Commit: commit}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&r)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var old commitChangeRecord
			if e := tx.Where("event_id = ?", id).Take(&old).Error; e != nil {
				return e
			}
			if old.Commit != commit {
				return fmt.Errorf("change already belongs to another commit")
			}
		}
	}
	return nil
}
func (s *Store) blobPath(ref string) (string, error) { return (content.Objects{Dir: s.Dir}).Path(ref) }
func (s *Store) PutBlob(data []byte) (string, error) { return (content.Objects{Dir: s.Dir}).Put(data) }
func (s *Store) Blob(ref string) ([]byte, error) {
	return (content.Objects{Dir: s.Dir}).Read(context.Background(), ref)
}
func (s *Store) Register(ctx context.Context, common, root, gitdir string) (w domain.Worktree, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		r := repositoryRecord{ID: domain.ID(), CommonDir: common}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "common_dir"}}, DoNothing: true}).Create(&r).Error; err != nil {
			return err
		}
		r = repositoryRecord{}
		if err := tx.Where("common_dir = ?", common).Take(&r).Error; err != nil {
			return err
		}
		row := worktreeRecord{ID: domain.ID(), RepositoryID: r.ID, Root: root, GitDir: gitdir}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "git_dir"}}, DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		// Clear the generated primary key before looking up an existing registration.
		row = worktreeRecord{}
		if err := tx.Where("git_dir = ?", gitdir).Take(&row).Error; err != nil {
			return err
		}
		w = domain.Worktree{ID: row.ID, RepositoryID: row.RepositoryID, Root: row.Root, GitDir: row.GitDir}
		return nil
	})
	return
}
func (s *Store) snapshot(id string) (domain.Snapshot, error) {
	var row snapshotRecord
	if e := s.db.Where("id = ?", id).Take(&row).Error; e != nil {
		return domain.Snapshot{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.nodes) > 100000 {
		s.nodes = map[string]domain.State{}
	}
	restored, e := state.Restore(row.Root, func(id string) ([]byte, error) { return s.Blob(id) }, s.nodes)
	return domain.Snapshot{ID: row.ID, WorktreeID: row.WorktreeID, Base: row.Base, Trigger: row.Trigger, Time: row.ObservedAt, State: restored}, e
}
func (s *Store) Snapshot(id string) (domain.Snapshot, error) {
	release, e := s.lease(false)
	if e != nil {
		return domain.Snapshot{}, e
	}
	defer release()
	return s.snapshot(id)
}
func (s *Store) saveSnapshot(tx *gorm.DB, w, base, trigger string, state domain.State, prepared *PreparedStates) (domain.Snapshot, error) {
	sn := domain.Snapshot{ID: domain.ID(), WorktreeID: w, Base: base, Trigger: trigger, Time: domain.Now(), State: state}
	if !prepared.roots[state.Root()] {
		return sn, fmt.Errorf("state root was not prepared")
	}
	row := snapshotRecord{ID: sn.ID, WorktreeID: w, Base: base, Trigger: trigger, ObservedAt: sn.Time, Root: state.Root()}
	if e := tx.Create(&row).Error; e != nil {
		return sn, e
	}

	return sn, nil
}
func cursor(q *gorm.DB, w string) (domain.Cursor, error) {
	var row cursorRecord
	if err := q.Where("worktree_id = ?", w).Take(&row).Error; err != nil {
		return domain.Cursor{}, err
	}
	return domain.Cursor{SnapshotID: row.SnapshotID, Base: row.Base, ActiveCall: row.ActiveCall, Segment: row.Segment, Version: row.Version}, nil
}
func (s *Store) Cursor(w string) (domain.Cursor, error) { return cursor(s.db, w) }

func ContentHash(data []byte) string { return content.Hash(data) }

// TransactionStats measures time spent executing transaction bodies, excluding
// acquisition wait and commit. It is diagnostic, not a correctness threshold.
func (s *Store) TransactionStats() (count, workNanos int64) {
	return s.transactions.Load(), s.transactionWork.Load()
}
