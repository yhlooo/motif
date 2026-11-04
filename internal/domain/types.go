// Package domain defines Motif facts, immutable state values and publication plans.
package domain

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/yhlooo/motif/internal/domain/state"
	"iter"
	"time"
)

func ID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func Now() int64 { return time.Now().UTC().UnixMicro() }

// Shared state values are independent of persistence and transport.
type File = state.File
type State = state.State

func ChangedPaths(a, b State) iter.Seq[string]          { return state.ChangedPaths(a, b) }
func SameFile(a, b File) bool                           { return state.SameFile(a, b) }
func From(m map[string]File) State                      { return state.From(m) }
func DisplayPath(p string) string                       { return state.DisplayPath(p) }
func Equal(a, b State) bool                             { return state.Equal(a, b) }
func Clone(s State) State                               { return state.Clone(s) }
func Apply(current, before, after State) (State, error) { return state.Apply(current, before, after) }

type Repository struct{ ID, CommonDir string }
type Worktree struct{ ID, RepositoryID, Root, GitDir string }
type Snapshot struct {
	ID, WorktreeID, Base, Trigger string
	Time                          int64
	State                         State
}
type Call struct {
	ID           string          `json:"id"`
	TurnID       string          `json:"turn_id"`
	Session      string          `json:"session"`
	ExternalID   string          `json:"external_id"`
	Tool         string          `json:"tool"`
	Model        string          `json:"model,omitempty"`
	Agent        string          `json:"agent,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	ReasonSource string          `json:"reason_source,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	UserName     string          `json:"git_user_name,omitempty"`
	UserEmail    string          `json:"git_user_email,omitempty"`
	Status       string          `json:"status"`
}
type Event struct {
	Origins          []string `json:"origin_event_ids,omitempty"`
	Projected        bool     `json:"projected,omitempty"`
	ID               string   `json:"id"`
	WorktreeID       string   `json:"worktree_id"`
	Sequence         int64    `json:"sequence"`
	Kind             string   `json:"kind"`
	Segment          string   `json:"segment"`
	Base             string   `json:"base_commit"`
	Commit           string   `json:"final_commit,omitempty"`
	TurnID           string   `json:"turn_id,omitempty"`
	CallID           string   `json:"call_id,omitempty"`
	BeforeID         string   `json:"before_snapshot"`
	AfterID          string   `json:"after_snapshot"`
	PairID           string   `json:"pair_id,omitempty"`
	From             int64    `json:"observed_from"`
	Until            int64    `json:"observed_until"`
	BatchID          string   `json:"batch_id,omitempty"`
	BatchTitle       string   `json:"batch_title,omitempty"`
	BatchDescription string   `json:"batch_description,omitempty"`
	Call             *Call    `json:"call,omitempty"`
}
type Batch struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Members     []string `json:"members"`
}
type Cursor struct {
	SnapshotID, Base, ActiveCall, Segment string
	Version                               int64
}
type Observation struct {
	KeepCall                    bool
	ResumePair                  string
	Worktree                    Worktree
	Base                        string
	State                       State
	BaseState                   State
	Trigger                     string
	Session, Turn, ExternalCall string
	Call                        *Call
	// Commit signals a verified direct-child commit. Caller identity proves an active tool context.
	Commit                  string
	CommitState             State
	Parent                  string
	CallerSession, CallerID string
	Reset                   bool
	ExpectedVersion         int64
	Receipt, Digest         string
}
type RefreshState struct {
	Attempt       int64  `json:"attempt"`
	AttemptedAt   int64  `json:"attempted_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	Code          string `json:"code"`
	Message       string `json:"message,omitempty"`
}
type Diagnostic struct {
	Code    string   `json:"code"`
	Stage   string   `json:"stage"`
	EventID string   `json:"event_id,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Message string   `json:"message"`
}
type Review struct {
	ProjectionID   string       `json:"projection_id"`
	Closure        string       `json:"closure"`
	InputSignature string       `json:"-"`
	ComparisonMode string       `json:"comparison_mode"`
	History        string       `json:"history"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
	Refresh        RefreshState `json:"refresh"`
	CheckSource    bool         `json:"-"`
	SourceRevision int64        `json:"-"`
	ViewDigest     string       `json:"-"`
	Version        int64        `json:"view_version"`
	ID             string       `json:"id"`
	RepositoryID   string       `json:"repository_id"`
	Title          string       `json:"title"`
	Source         string       `json:"source"`
	Target         string       `json:"target"`
	SourceOID      string       `json:"source_oid"`
	TargetOID      string       `json:"target_oid"`
	Base           string       `json:"base"`
	Head           string       `json:"head"`
	Created        int64        `json:"created"`
	Status         string       `json:"status"`
	Integrity      string       `json:"integrity"`
	Warning        string       `json:"warning,omitempty"`
	Commits        []string     `json:"commits"`
	Items          []ReviewItem `json:"items"`

	CommitTitles map[string]string `json:"commit_titles,omitempty"`
}
type ReviewItem struct {
	DecisionRevision  int64    `json:"decision_revision"`
	BeforeID, AfterID string   `json:"-"`
	StableKey         string   `json:"stable_key"`
	Commit            string   `json:"commit_oid"`
	SourceBatch       string   `json:"source_batch_uid,omitempty"`
	Digest            string   `json:"content_digest"`
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Decision          string   `json:"decision"`
	Events            []string `json:"events"`
	Before            State    `json:"-"`
	After             State    `json:"-"`
}
type Thread struct {
	AnchorItem  string    `json:"anchor_item"`
	BeforeID    string    `json:"before_snapshot"`
	AfterID     string    `json:"after_snapshot"`
	MappedStart int       `json:"mapped_start,omitempty"`
	MappedEnd   int       `json:"mapped_end,omitempty"`
	MappedPath  string    `json:"mapped_path,omitempty"`
	ID          string    `json:"id"`
	ReviewID    string    `json:"review_id"`
	First       string    `json:"first"`
	Last        string    `json:"last"`
	Path        string    `json:"path"`
	Side        string    `json:"side"`
	Start       int       `json:"start"`
	End         int       `json:"end"`
	ContentRef  string    `json:"content_ref"`
	Comments    []Comment `json:"comments"`
}
type Comment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Created int64  `json:"created"`
}
