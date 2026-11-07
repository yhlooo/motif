package content

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

// Lease coordinates readers, publishers and deletion across processes sharing
// this data directory. Each call owns a separate open file description.
func (o Objects) Lease(exclusive bool) (func(), error) {
	return o.LeaseContext(context.Background(), exclusive)
}

func (o Objects) LeaseContext(ctx context.Context, exclusive bool) (func(), error) {
	f, e := os.OpenFile(filepath.Join(o.Dir, "lifecycle.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	for {
		if e = ctx.Err(); e != nil {
			f.Close()
			return nil, e
		}
		e = unix.Flock(int(f.Fd()), mode|unix.LOCK_NB)
		if e == nil {
			break
		}
		if !errors.Is(e, unix.EWOULDBLOCK) && !errors.Is(e, unix.EAGAIN) && !errors.Is(e, unix.EINTR) {
			f.Close()
			return nil, e
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}
