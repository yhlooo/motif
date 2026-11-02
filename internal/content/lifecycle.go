package content

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

// Lease coordinates readers, publishers and deletion across processes sharing
// this data directory. Each call owns a separate open file description.
func (o Objects) Lease(exclusive bool) (func(), error) {
	f, e := os.OpenFile(filepath.Join(o.Dir, "lifecycle.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	mode := unix.LOCK_SH
	if exclusive {
		mode = unix.LOCK_EX
	}
	if e = unix.Flock(int(f.Fd()), mode); e != nil {
		f.Close()
		return nil, e
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}
