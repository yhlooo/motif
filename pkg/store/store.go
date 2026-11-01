package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var ErrNotFound = gorm.ErrRecordNotFound

type Store struct {
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
	s := &Store{db: db, Dir: dir}
	if err = s.migrate(); err != nil {
		_ = pool.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) migrate() error {
	// ORM-only refactoring does not change the v1 schema or rewrite existing data.
	if s.db.Migrator().HasTable(&schemaMigration{}) {
		var version schemaMigration
		if err := s.db.Take(&version).Error; err != nil {
			return err
		}
		if version.Version != 1 {
			return fmt.Errorf("unsupported database schema %d", version.Version)
		}
		for _, model := range models() {
			if !s.db.Migrator().HasTable(model) {
				return fmt.Errorf("incomplete database schema: missing table %s", model.(interface{ TableName() string }).TableName())
			}
		}
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(models()...); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&schemaMigration{Version: 1}).Error
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
	return s.db.WithContext(ctx).Transaction(fn)
}
func (s *Store) blobPath(hash string) (string, error) {
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid blob hash")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, "blobs", "sha256", hash[:2], hash[2:]), nil
}
func (s *Store) PutBlob(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	p, _ := s.blobPath(hash)
	if old, err := os.ReadFile(p); err == nil {
		if sha256.Sum256(old) != sum {
			return "", fmt.Errorf("corrupt blob %s", hash)
		}
		return hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".blob-")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), p); err != nil {
		return "", err
	}
	d, err := os.Open(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	err = d.Sync()
	d.Close()
	return hash, err
}
func (s *Store) Blob(hash string) ([]byte, error) {
	p, err := s.blobPath(hash)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("missing blob %s: %w", hash, err)
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != hash {
		return nil, fmt.Errorf("corrupt blob %s", hash)
	}
	return b, nil
}

func (s *Store) Register(ctx context.Context, common, root, gitdir string) (w Worktree, err error) {
	err = s.update(ctx, func(tx *gorm.DB) error {
		r := repositoryRecord{ID: ID(), CommonDir: common}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "common_dir"}}, DoNothing: true}).Create(&r).Error; err != nil {
			return err
		}
		r = repositoryRecord{}
		if err := tx.Where("common_dir = ?", common).Take(&r).Error; err != nil {
			return err
		}
		row := worktreeRecord{ID: ID(), RepositoryID: r.ID, Root: root, GitDir: gitdir}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "git_dir"}}, DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		// Clear the generated primary key before looking up an existing registration.
		row = worktreeRecord{}
		if err := tx.Where("git_dir = ?", gitdir).Take(&row).Error; err != nil {
			return err
		}
		w = Worktree{ID: row.ID, RepositoryID: row.RepositoryID, Root: row.Root, GitDir: row.GitDir}
		return nil
	})
	return
}
func snapshot(q *gorm.DB, id string) (Snapshot, error) {
	var row snapshotRecord
	if err := q.Where("id = ?", id).Take(&row).Error; err != nil {
		return Snapshot{}, err
	}
	sn := Snapshot{ID: row.ID, WorktreeID: row.WorktreeID, Base: row.Base, Trigger: row.Trigger, Time: row.ObservedAt, State: State{}}
	var entries []snapshotEntryRecord
	if err := q.Where("snapshot_id = ?", id).Find(&entries).Error; err != nil {
		return sn, err
	}
	for _, entry := range entries {
		sn.State[string(entry.Path)] = File{Mode: entry.Mode, Hash: entry.Hash, OID: entry.OID, OutOfScope: entry.OutOfScope}
	}
	return sn, nil
}
func (s *Store) Snapshot(id string) (Snapshot, error) {
	sn, err := snapshot(s.db, id)
	if err == nil {
		err = s.validateState(sn.State)
	}
	return sn, err
}
func (s *Store) validateState(state State) error {
	for _, f := range state {
		if f.Hash != "" {
			if _, err := s.Blob(f.Hash); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveSnapshot(tx *gorm.DB, w, base, trigger string, state State) (Snapshot, error) {
	sn := Snapshot{ID: ID(), WorktreeID: w, Base: base, Trigger: trigger, Time: Now(), State: state}
	row := snapshotRecord{ID: sn.ID, WorktreeID: w, Base: base, Trigger: trigger, ObservedAt: sn.Time}
	if err := tx.Create(&row).Error; err != nil {
		return sn, err
	}
	entries := make([]snapshotEntryRecord, 0, len(state))
	for p, f := range state {
		entries = append(entries, snapshotEntryRecord{SnapshotID: sn.ID, Path: []byte(p), Mode: f.Mode, Hash: f.Hash, OID: f.OID, OutOfScope: f.OutOfScope})
	}
	if len(entries) > 0 {
		if err := tx.CreateInBatches(entries, 100).Error; err != nil {
			return sn, err
		}
	}
	return sn, nil
}
func cursor(q *gorm.DB, w string) (Cursor, error) {
	var row cursorRecord
	if err := q.Where("worktree_id = ?", w).Take(&row).Error; err != nil {
		return Cursor{}, err
	}
	return Cursor{SnapshotID: row.SnapshotID, Base: row.Base, ActiveCall: row.ActiveCall, Segment: row.Segment, Version: row.Version}, nil
}
func (s *Store) Cursor(w string) (Cursor, error) { return cursor(s.db, w) }
