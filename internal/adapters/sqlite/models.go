package store

import (
	"encoding/json"
	"github.com/yhlooo/motif/internal/domain"
)

// Persistence models keep database keys/constraints independent of API views.
// Schema is intentionally incompatible with the pre-rebuild development databases.
type schemaMigration struct {
	Version int `gorm:"primaryKey;autoIncrement:false"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

type repositoryRecord struct {
	ID         string `gorm:"primaryKey"`
	CommonDir  string `gorm:"not null;uniqueIndex"`
	Generation int64  `gorm:"not null"`
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
func (r callRecord) value() domain.Call {
	return domain.Call{ID: r.ID, TurnID: r.TurnID, Session: r.Session, ExternalID: r.ExternalID, Tool: r.Tool, Model: r.Model, Agent: r.Agent, Reason: r.Reason, ReasonSource: r.ReasonSource, Input: r.Input, UserName: r.UserName, UserEmail: r.UserEmail, Status: r.Status}
}
func callRow(c domain.Call) callRecord {
	return callRecord{ID: c.ID, TurnID: c.TurnID, Session: c.Session, ExternalID: c.ExternalID, Tool: c.Tool, Model: c.Model, Agent: c.Agent, Reason: c.Reason, ReasonSource: c.ReasonSource, Input: []byte(c.Input), UserName: c.UserName, UserEmail: c.UserEmail, Status: c.Status}
}

type snapshotRecord struct {
	Root          string          `gorm:"not null"`
	ID            string          `gorm:"primaryKey"`
	WorktreeID    string          `gorm:"not null;index"`
	Base, Trigger string          `gorm:"not null"`
	ObservedAt    int64           `gorm:"not null"`
	Worktree      *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
}

func (snapshotRecord) TableName() string { return "snapshots" }

type cursorRecord struct {
	WorktreeID                            string          `gorm:"primaryKey"`
	SnapshotID, Base, ActiveCall, Segment string          `gorm:"not null"`
	Version                               int64           `gorm:"not null"`
	Worktree                              *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
	Snapshot                              *snapshotRecord `gorm:"foreignKey:SnapshotID"`
}

func (cursorRecord) TableName() string { return "worktree_cursors" }

type eventRecord struct {
	Projected                                 bool            `gorm:"not null;uniqueIndex:event_sequence,priority:2"`
	ID                                        string          `gorm:"primaryKey"`
	WorktreeID                                string          `gorm:"not null;uniqueIndex:event_sequence,priority:1"`
	Sequence                                  int64           `gorm:"not null;uniqueIndex:event_sequence,priority:3"`
	Kind, Segment, Base                       string          `gorm:"not null"`
	FinalCommit                               string          `gorm:"-"`
	TurnID, CallID, BeforeID, AfterID, PairID string          `gorm:"not null"`
	IntervalStart, IntervalEnd                int64           `gorm:"not null"`
	Worktree                                  *worktreeRecord `gorm:"foreignKey:WorktreeID;constraint:OnDelete:CASCADE"`
	Before                                    *snapshotRecord `gorm:"foreignKey:BeforeID"`
	After                                     *snapshotRecord `gorm:"foreignKey:AfterID"`
}

func (eventRecord) TableName() string { return "edit_events" }
func (r eventRecord) value() domain.Event {
	return domain.Event{Projected: r.Projected, ID: r.ID, WorktreeID: r.WorktreeID, Sequence: r.Sequence, Kind: r.Kind, Segment: r.Segment, Base: r.Base, Commit: r.FinalCommit, TurnID: r.TurnID, CallID: r.CallID, BeforeID: r.BeforeID, AfterID: r.AfterID, PairID: r.PairID, From: r.IntervalStart, Until: r.IntervalEnd}
}

type batchRecord struct {
	OriginBatch              string          `gorm:"not null;default:''"`
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
	ProjectionID                      string            `gorm:"not null"`
	InputSignature                    string            `gorm:"not null;default:''"`
	ComparisonMode                    string            `gorm:"not null;default:'merge_base'"`
	History                           string            `gorm:"not null;default:'unavailable'"`
	Diagnostics                       string            `gorm:"not null;default:'[]'"`
	RefreshAttempt                    int64             `gorm:"not null;default:0"`
	RefreshAttemptedAt                int64             `gorm:"not null;default:0"`
	RefreshSuccessAt                  int64             `gorm:"not null;default:0"`
	RefreshCode                       string            `gorm:"not null;default:''"`
	RefreshMessage                    string            `gorm:"not null;default:''"`
	SourceRevision                    int64             `gorm:"not null;default:0"`
	ViewDigest                        string            `gorm:"not null;default:''"`
	Version                           int64             `gorm:"not null;default:1"`
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
func (r reviewRecord) value() domain.Review {
	var diagnostics []domain.Diagnostic
	_ = json.Unmarshal([]byte(r.Diagnostics), &diagnostics)
	closure := ""
	if r.Integrity == "verified" || r.Integrity == "completed_with_unknown" {
		closure = "verified"
	}
	return domain.Review{Closure: closure, InputSignature: r.InputSignature, SourceRevision: r.SourceRevision, ComparisonMode: r.ComparisonMode, History: r.History, Diagnostics: diagnostics, Refresh: domain.RefreshState{Attempt: r.RefreshAttempt, AttemptedAt: r.RefreshAttemptedAt, LastSuccessAt: r.RefreshSuccessAt, Code: r.RefreshCode, Message: r.RefreshMessage}, ViewDigest: r.ViewDigest, Version: r.Version, ID: r.ID, RepositoryID: r.RepositoryID, Title: r.Title, Source: r.Source, Target: r.Target, SourceOID: r.SourceOID, TargetOID: r.TargetOID, Base: r.Base, Head: r.Head, Created: r.Created, Integrity: r.Integrity, Warning: r.Warning}
}

// Stable identity belongs to a review; presentation belongs to an immutable projection.
type reviewItemRecord struct {
	ID        string        `gorm:"primaryKey"`
	ReviewID  string        `gorm:"not null;uniqueIndex:review_identity,priority:1"`
	StableKey string        `gorm:"not null;uniqueIndex:review_identity,priority:2"`
	Review    *reviewRecord `gorm:"foreignKey:ReviewID;constraint:OnDelete:CASCADE"`
}

func (reviewItemRecord) TableName() string { return "review_item_identities" }

type decisionRecord struct {
	ItemID   string            `gorm:"primaryKey"`
	Decision string            `gorm:"not null;check:decision IN ('pending','approved','rejected')"`
	Revision int64             `gorm:"not null"`
	Item     *reviewItemRecord `gorm:"foreignKey:ItemID;constraint:OnDelete:CASCADE"`
}

func (decisionRecord) TableName() string { return "review_decisions" }

type reviewProjectionRecord struct {
	ID       string        `gorm:"primaryKey"`
	ReviewID string        `gorm:"not null;uniqueIndex:projection_version,priority:1"`
	Version  int64         `gorm:"not null;uniqueIndex:projection_version,priority:2"`
	Payload  []byte        `gorm:"not null"`
	Review   *reviewRecord `gorm:"foreignKey:ReviewID;constraint:OnDelete:CASCADE"`
}

func (reviewProjectionRecord) TableName() string { return "review_projections" }

type threadRecord struct {
	AnchorItem                    string            `gorm:"not null;default:''"`
	BeforeID                      string            `gorm:"not null;default:''"`
	AfterID                       string            `gorm:"not null;default:''"`
	ID                            string            `gorm:"primaryKey"`
	ReviewID, FirstItem, LastItem string            `gorm:"not null"`
	Path                          []byte            `gorm:"type:blob;not null"`
	Side                          string            `gorm:"not null;check:side IN ('old','new')"`
	StartLine                     int               `gorm:"not null;check:start_line>=1"`
	EndLine                       int               `gorm:"not null;check:end_line>=start_line"`
	ContentRef                    string            `gorm:"not null"`
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
	return []any{&schemaMigration{}, &objectReference{}, &deletionTask{}, &commitChangeRecord{}, &derivationRecord{}, &repositoryRecord{}, &worktreeRecord{}, &turnRecord{}, &callRecord{}, &snapshotRecord{}, &cursorRecord{}, &eventRecord{}, &batchRecord{}, &batchMemberRecord{}, &receiptRecord{}, &reviewRecord{}, &reviewItemRecord{}, &decisionRecord{}, &reviewProjectionRecord{}, &threadRecord{}, &commentRecord{}, &rewriteRecord{}, &projectionRecord{}, &referenceRecord{}}
}

// Projections identify derived events without changing any original observation.
type rewriteRecord struct {
	ID           string            `gorm:"primaryKey"`
	RepositoryID string            `gorm:"not null;uniqueIndex:rewrite_identity,priority:1"`
	Kind         string            `gorm:"not null"`
	Old          string            `gorm:"not null;uniqueIndex:rewrite_identity,priority:2"`
	New          string            `gorm:"not null;uniqueIndex:rewrite_identity,priority:3"`
	Position     int               `gorm:"not null"`
	Repository   *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (rewriteRecord) TableName() string { return "commit_rewrites" }

type projectionRecord struct {
	ID           string            `gorm:"primaryKey"`
	RepositoryID string            `gorm:"not null;uniqueIndex:commit_projection,priority:1"`
	Commit       string            `gorm:"column:commit_oid;not null;uniqueIndex:commit_projection,priority:2"`
	Parent       string            `gorm:"not null"`
	Warning      string            `gorm:"not null"`
	Repository   *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (projectionRecord) TableName() string { return "commit_projections" }

type referenceRecord struct {
	ID           string            `gorm:"primaryKey"`
	RepositoryID string            `gorm:"not null;index"`
	Old, New     string            `gorm:"not null"`
	Ref          []byte            `gorm:"not null;type:blob"`
	ObservedAt   int64             `gorm:"not null"`
	Repository   *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (referenceRecord) TableName() string { return "reference_transitions" }

// Associations are append-only; observations never acquire mutable commit fields.
type commitChangeRecord struct {
	EventID string       `gorm:"primaryKey"`
	Commit  string       `gorm:"not null;index"`
	Event   *eventRecord `gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE"`
}

func (commitChangeRecord) TableName() string { return "commit_changes" }

type objectReference struct {
	RepositoryID string            `gorm:"primaryKey"`
	Hash         string            `gorm:"primaryKey"`
	Repository   *repositoryRecord `gorm:"foreignKey:RepositoryID;constraint:OnDelete:CASCADE"`
}

func (objectReference) TableName() string { return "object_refs" }

type deletionTask struct {
	Hash string `gorm:"primaryKey"`
}

func (deletionTask) TableName() string { return "pending_object_deletes" }

type derivationRecord struct {
	ChangeID string       `gorm:"primaryKey"`
	OriginID string       `gorm:"primaryKey"`
	Change   *eventRecord `gorm:"foreignKey:ChangeID;constraint:OnDelete:CASCADE"`
	Origin   *eventRecord `gorm:"foreignKey:OriginID;constraint:OnDelete:CASCADE"`
}

func (derivationRecord) TableName() string { return "rewrite_derivations" }
