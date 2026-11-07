package gitrepo

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/yhlooo/motif/internal/domain"
)

type cachedInfo struct {
	ctime int64
	size  int64
}

func (i cachedInfo) Name() string       { return "file" }
func (i cachedInfo) Size() int64        { return i.size }
func (i cachedInfo) Mode() os.FileMode  { return 0600 }
func (i cachedInfo) ModTime() time.Time { return time.Unix(1, 0) }
func (i cachedInfo) IsDir() bool        { return false }
func (i cachedInfo) Sys() any {
	return &syscall.Stat_t{Dev: 1, Ino: 2, Ctim: syscall.Timespec{Sec: i.ctime}}
}
func TestCachedContentNeedsStableIdentityAndNonRacyTimestamps(t *testing.T) {
	f := domain.File{Mode: "100644", ContentRef: "blob", ContentID: "identity"}
	previous := []domain.State{domain.From(map[string]domain.File{"raw/\xff": f})}
	info := cachedInfo{ctime: 1, size: 3}
	cache := fileCache{ValidatedAt: 10 * int64(time.Second), Files: map[string]fileStamp{cacheKey("raw/\xff"): stamp(info)}}
	if got, ok := cachedFile("raw/\xff", info, cache, previous); !ok || got != f {
		t.Fatal("stable file was not reused")
	}
	if _, ok := cachedFile("raw/\xff", cachedInfo{ctime: 2, size: 3}, cache, previous); ok {
		t.Fatal("same-size edit reused")
	}
	cache.ValidatedAt = 2 * int64(time.Second)
	if _, ok := cachedFile("raw/\xff", info, cache, previous); ok {
		t.Fatal("racy timestamp reused")
	}
}
