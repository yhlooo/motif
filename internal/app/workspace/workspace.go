// Package workspace composes independent Git, object and metadata adapters.
package workspace

import (
	"context"
	"github.com/yhlooo/motif/internal/adapters/gitrepo"
	"github.com/yhlooo/motif/internal/adapters/sqlite"
	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
)

type Project struct {
	*gitrepo.Repository
	Store *sqlite.Store
}

func Open(ctx context.Context, dir string, s *sqlite.Store) (*Project, error) {
	r, e := gitrepo.Open(ctx, dir, content.Objects{Dir: s.Dir})
	if e != nil {
		return nil, e
	}
	return &Project{Repository: r, Store: s}, nil
}
func (p *Project) Identity(ctx context.Context) (domain.Worktree, error) {
	return p.Store.Register(ctx, p.CommonDir, p.Root, p.GitDir)
}
