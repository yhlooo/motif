package store

// Persistence models keep database keys/constraints independent of API views.
// Table and column names retain the original v1 schema for existing databases.
type schemaMigration struct {
	Version int `gorm:"primaryKey;autoIncrement:false"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

type repositoryRecord struct {
	ID        string `gorm:"primaryKey"`
	CommonDir string `gorm:"not null;uniqueIndex"`
}

func (repositoryRecord) TableName() string { return "repositories" }

type worktreeRecord struct {
	ID           string            `gorm:"primaryKey"`
	RepositoryID string            `gorm:"not null;index"`
	Root         string            `gorm:"not null"`
	GitDir       string            `gorm:"not null;uniqueIndex"`
	Repository   *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (worktreeRecord) TableName() string { return "worktrees" }

type turnRecord struct {
	ID         string          `gorm:"primaryKey"`
	WorktreeID string          `gorm:"not null;uniqueIndex:turn_identity,priority:1"`
	Session    string          `gorm:"not null;uniqueIndex:turn_identity,priority:2"`
	ExternalID string          `gorm:"not null;uniqueIndex:turn_identity,priority:3"`
	Worktree   *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
}

func (turnRecord) TableName() string { return "turns" }

type callRecord struct {
	ID                                       string `gorm:"primaryKey"`
	TurnID                                   string `gorm:"not null;uniqueIndex:call_identity,priority:1"`
	Session                                  string `gorm:"not null"`
	ExternalID                               string `gorm:"not null;uniqueIndex:call_identity,priority:2"`
	Tool, Model, Agent, Reason, ReasonSource string `gorm:"not null"`
	Input                                    []byte
	UserName, UserEmail, Status              string      `gorm:"not null"`
	Turn                                     *turnRecord `gorm:"foreignKey:TurnID;constraint:OnDelete:CASCADE"`
}

func (callRecord) TableName() string { return "tool_calls" }
func (r callRecord) value() Call {
	return Call{ID: r.ID, TurnID: r.TurnID, Session: r.Session, ExternalID: r.ExternalID, Tool: r.Tool, Model: r.Model, Agent: r.Agent, Reason: r.Reason, ReasonSource: r.ReasonSource, Input: r.Input, UserName: r.UserName, UserEmail: r.UserEmail, Status: r.Status}
}
func callRow(c Call) callRecord {
	return callRecord{ID: c.ID, TurnID: c.TurnID, Session: c.Session, ExternalID: c.ExternalID, Tool: c.Tool, Model: c.Model, Agent: c.Agent, Reason: c.Reason, ReasonSource: c.ReasonSource, Input: []byte(c.Input), UserName: c.UserName, UserEmail: c.UserEmail, Status: c.Status}
}

type snapshotRecord struct {
	ID            string          `gorm:"primaryKey"`
	WorktreeID    string          `gorm:"not null;index"`
	Base, Trigger string          `gorm:"not null"`
	ObservedAt    int64           `gorm:"not null"`
	Worktree      *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
}

func (snapshotRecord) TableName() string { return "snapshots" }

type snapshotEntryRecord struct {
	SnapshotID string          `gorm:"primaryKey"`
	Path       []byte          `gorm:"type:blob;primaryKey"`
	Mode       string          `gorm:"not null"`
	Hash       string          `gorm:"not null;index:snapshot_hash"`
	OID        string          `gorm:"column:oid;not null"`
	OutOfScope bool            `gorm:"not null;default:false"`
	Snapshot   *snapshotRecord `gorm:"foreignKey:SnapshotID;constraint:OnDelete:CASCADE"`
}

func (snapshotEntryRecord) TableName() string { return "snapshot_entries" }

type cursorRecord struct {
	WorktreeID                            string          `gorm:"primaryKey"`
	SnapshotID, Base, ActiveCall, Segment string          `gorm:"not null"`
	Version                               int64           `gorm:"not null"`
	Worktree                              *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
	Snapshot                              *snapshotRecord `gorm:"foreignKey:SnapshotID"`
}

func (cursorRecord) TableName() string { return "worktree_cursors" }

type eventRecord struct {
	ID                                        string          `gorm:"primaryKey"`
	WorktreeID                                string          `gorm:"not null;uniqueIndex:event_sequence,priority:1"`
	Sequence                                  int64           `gorm:"not null;uniqueIndex:event_sequence,priority:2"`
	Kind, Segment, Base                       string          `gorm:"not null"`
	FinalCommit                               string          `gorm:"not null;index:events_commit"`
	TurnID, CallID, BeforeID, AfterID, PairID string          `gorm:"not null"`
	IntervalStart, IntervalEnd                int64           `gorm:"not null"`
	Worktree                                  *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
	Before                                    *snapshotRecord `gorm:"foreignKey:BeforeID"`
	After                                     *snapshotRecord `gorm:"foreignKey:AfterID"`
}

func (eventRecord) TableName() string { return "edit_events" }
func (r eventRecord) value() Event {
	return Event{ID: r.ID, WorktreeID: r.WorktreeID, Sequence: r.Sequence, Kind: r.Kind, Segment: r.Segment, Base: r.Base, Commit: r.FinalCommit, TurnID: r.TurnID, CallID: r.CallID, BeforeID: r.BeforeID, AfterID: r.AfterID, PairID: r.PairID, From: r.IntervalStart, Until: r.IntervalEnd}
}

type batchRecord struct {
	ID                       string          `gorm:"primaryKey"`
	WorktreeID               string          `gorm:"not null;index"`
	Kind, Title, Description string          `gorm:"not null"`
	Worktree                 *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
}

func (batchRecord) TableName() string { return "patch_batches" }

type batchMemberRecord struct {
	BatchID  string       `gorm:"primaryKey"`
	Position int          `gorm:"primaryKey;autoIncrement:false"`
	EventID  string       `gorm:"not null;uniqueIndex"`
	Batch    *batchRecord `gorm:"foreignKey:BatchID;constraint:OnDelete:CASCADE"`
	Event    *eventRecord `gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}

func (batchMemberRecord) TableName() string { return "patch_batch_members" }

type receiptRecord struct {
	WorktreeID string          `gorm:"primaryKey"`
	Key        string          `gorm:"primaryKey"`
	Digest     string          `gorm:"not null"`
	Worktree   *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
}

func (receiptRecord) TableName() string { return "hook_receipts" }

type reviewRecord struct {
	ID                                string            `gorm:"primaryKey"`
	RepositoryID                      string            `gorm:"not null;index"`
	Title, Source, Target, Base, Head string            `gorm:"not null"`
	SourceOID                         string            `gorm:"column:source_oid;not null"`
	TargetOID                         string            `gorm:"column:target_oid;not null"`
	Created                           int64             `gorm:"not null"`
	Integrity, Warning                string            `gorm:"not null"`
	Repository                        *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (reviewRecord) TableName() string { return "code_reviews" }
func (r reviewRecord) value() Review {
	return Review{ID: r.ID, RepositoryID: r.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning}
}

type reviewCommitRecord struct {
	ReviewID string        `gorm:"primaryKey"`
	Position int           `gorm:"primaryKey;autoIncrement:false"`
	OID      string        `gorm:"column:oid;not null"`
	Review   *reviewRecord `gorm:"foreignKey:ReviewID;constraint:OnDelete:CASCADE"`
}

func (reviewCommitRecord) TableName() string { return "code_review_commits" }

type reviewItemRecord struct {
	ID                       string          `gorm:"primaryKey"`
	ReviewID                 string          `gorm:"not null;uniqueIndex:review_position,priority:1"`
	Position                 int             `gorm:"not null;uniqueIndex:review_position,priority:2"`
	Kind, Title, Description string          `gorm:"not null"`
	Decision                 string          `gorm:"not null;check:decision IN ('pending','approved','rejected')"`
	BeforeID, AfterID        string          `gorm:"not null"`
	Review                   *reviewRecord   `gorm:"foreignKey:ReviewID;constraint:OnDelete:CASCADE"`
	Before                   *snapshotRecord `gorm:"foreignKey:BeforeID"`
	After                    *snapshotRecord `gorm:"foreignKey:AfterID"`
}

func (reviewItemRecord) TableName() string { return "code_review_items" }

type reviewEventRecord struct {
	ItemID   string            `gorm:"primaryKey"`
	Position int               `gorm:"primaryKey;autoIncrement:false"`
	EventID  string            `gorm:"not null"`
	Item     *reviewItemRecord `gorm:"foreignKey:ItemID;constraint:OnDelete:CASCADE"`
	Event    *eventRecord      `gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}

func (reviewEventRecord) TableName() string { return "code_review_item_events" }

type threadRecord struct {
	ID                            string            `gorm:"primaryKey"`
	ReviewID, FirstItem, LastItem string            `gorm:"not null"`
	Path                          []byte            `gorm:"type:blob;not null"`
	Side                          string            `gorm:"not null;check:side IN ('old','new')"`
	StartLine                     int               `gorm:"not null;check:start_line>=1"`
	EndLine                       int               `gorm:"not null;check:end_line>=start_line"`
	Hash                          string            `gorm:"not null"`
	Review                        *reviewRecord     `gorm:"foreignKey:ReviewID;constraint:OnDelete:CASCADE"`
	First                         *reviewItemRecord `gorm:"foreignKey:FirstItem"`
	Last                          *reviewItemRecord `gorm:"foreignKey:LastItem"`
}

func (threadRecord) TableName() string { return "comment_threads" }

type commentRecord struct {
	ID                     string        `gorm:"primaryKey"`
	ThreadID, Body, Author string        `gorm:"not null"`
	Created                int64         `gorm:"not null"`
	Thread                 *threadRecord `gorm:"foreignKey:ThreadID;constraint:OnDelete:CASCADE"`
}

func (commentRecord) TableName() string { return "comments" }
func models() []any {
	return []any{&schemaMigration{}, &repositoryRecord{}, &worktreeRecord{}, &turnRecord{}, &callRecord{}, &snapshotRecord{}, &snapshotEntryRecord{}, &cursorRecord{}, &eventRecord{}, &batchRecord{}, &batchMemberRecord{}, &receiptRecord{}, &reviewRecord{}, &reviewCommitRecord{}, &reviewItemRecord{}, &reviewEventRecord{}, &threadRecord{}, &commentRecord{}}
}
