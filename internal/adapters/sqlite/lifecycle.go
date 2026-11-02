package store

import "github.com/yhlooo/motif/internal/content"

func (s *Store) lease(exclusive bool) (func(), error) {
	return (content.Objects{Dir: s.Dir}).Lease(exclusive)
}
