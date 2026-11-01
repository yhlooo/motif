// Package store owns Motif's local database and immutable content library.
package store

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"
)

func ID() string { return fmt.Sprintf("%x", rand.Text()) }
func Now() int64 { return time.Now().UTC().UnixMicro() }

type File struct {
	OutOfScope bool   `json:"out_of_scope,omitempty"`
	Mode       string `json:"mode"`
	Hash       string `json:"hash,omitempty"`
	OID        string `json:"oid,omitempty"`
}

// State keys are raw Git path bytes. Its JSON encoding preserves non-UTF-8 paths.
type State map[string]File

func (s State) MarshalJSON() ([]byte, error) {
	m := map[string]File{}
	for p, f := range s {
		m[base64.StdEncoding.EncodeToString([]byte(p))] = f
	}
	return json.Marshal(m)
}
func (s *State) UnmarshalJSON(b []byte) error {
	var m map[string]File
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*s = State{}
	for p, f := range m {
		raw, err := base64.StdEncoding.DecodeString(p)
		if err != nil {
			return err
		}
		(*s)[string(raw)] = f
	}
	return nil
}
func DisplayPath(p string) string {
	if utf8.ValidString(p) {
		return p
	}
	return fmt.Sprintf("%q", []byte(p))
}
func Equal(a, b State) bool {
	if len(a) != len(b) {
		return false
	}
	for p, f := range a {
		if g, ok := b[p]; !ok || g != f {
			return false
		}
	}
	return true
}
func Clone(s State) State {
	c := State{}
	for p, f := range s {
		c[p] = f
	}
	return c
}

// Apply uses exact file-state preconditions, rather than concatenating diff text.
func Apply(current, before, after State) (State, error) {
	out := Clone(current)
	paths := map[string]bool{}
	for p := range before {
		paths[p] = true
	}
	for p := range after {
		paths[p] = true
	}
	for p := range paths {
		b, bok := before[p]
		a, aok := after[p]
		if bok == aok && b == a {
			continue
		}
		c, cok := current[p]
		if cok != bok || c != b {
			return nil, fmt.Errorf("cannot replay path %s: recorded before state differs", DisplayPath(p))
		}
		if aok {
			out[p] = a
		} else {
			delete(out, p)
		}
	}
	return out, nil
}

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
	ID               string `json:"id"`
	WorktreeID       string `json:"worktree_id"`
	Sequence         int64  `json:"sequence"`
	Kind             string `json:"kind"`
	Segment          string `json:"segment"`
	Base             string `json:"base_commit"`
	Commit           string `json:"final_commit,omitempty"`
	TurnID           string `json:"turn_id,omitempty"`
	CallID           string `json:"call_id,omitempty"`
	BeforeID         string `json:"before_snapshot"`
	AfterID          string `json:"after_snapshot"`
	PairID           string `json:"pair_id,omitempty"`
	From             int64  `json:"observed_from"`
	Until            int64  `json:"observed_until"`
	BatchID          string `json:"batch_id,omitempty"`
	BatchTitle       string `json:"batch_title,omitempty"`
	BatchDescription string `json:"batch_description,omitempty"`
	Call             *Call  `json:"call,omitempty"`
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
type Review struct {
	ID           string       `json:"id"`
	RepositoryID string       `json:"repository_id"`
	Title        string       `json:"title"`
	Source       string       `json:"source"`
	Target       string       `json:"target"`
	SourceOID    string       `json:"source_oid"`
	TargetOID    string       `json:"target_oid"`
	Base         string       `json:"base"`
	Head         string       `json:"head"`
	Created      int64        `json:"created"`
	Status       string       `json:"status"`
	Integrity    string       `json:"integrity"`
	Warning      string       `json:"warning,omitempty"`
	Commits      []string     `json:"commits"`
	Items        []ReviewItem `json:"items"`
}
type ReviewItem struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Decision    string   `json:"decision"`
	Events      []string `json:"events"`
	Before      State    `json:"-"`
	After       State    `json:"-"`
}
type Thread struct {
	ID       string    `json:"id"`
	ReviewID string    `json:"review_id"`
	First    string    `json:"first"`
	Last     string    `json:"last"`
	Path     string    `json:"path"`
	Side     string    `json:"side"`
	Start    int       `json:"start"`
	End      int       `json:"end"`
	Hash     string    `json:"hash"`
	Comments []Comment `json:"comments"`
}
type Comment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Author  string `json:"author"`
	Created int64  `json:"created"`
}
