package domain

import (
	"crypto/sha256"
	"fmt"
)

type RewriteMapping struct{ Old, New string }
type ProjectedChange struct {
	Origin        Event
	Before, After State
	Kind, PairID  string
}
type CommitProjection struct {
	Commit, Parent, Warning string
	Changes                 []ProjectedChange
}

func ProjectionBatchID(commit, origin string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(commit+"/"+origin)))
}

type RewritePlan struct {
	Worktree                       Worktree
	Kind                           string
	Mappings                       []RewriteMapping
	Projections                    []CommitProjection
	Pending                        []string
	ExpectedCursor, SourceRevision int64
	Resume                         Observation
}
