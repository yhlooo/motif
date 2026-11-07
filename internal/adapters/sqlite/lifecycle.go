package sqlite

import (
	"context"

	"github.com/yhlooo/motif/internal/content"
)

func (s *Store) lease(exclusive bool) (func(), error) {
	return (content.Objects{Dir: s.Dir}).Lease(exclusive)
}

// CaptureLease protects content preparation through publication from deletion.
func (s *Store) CaptureLease(ctx context.Context) (func(), error) {
	return (content.Objects{Dir: s.Dir}).LeaseContext(ctx, false)
}
