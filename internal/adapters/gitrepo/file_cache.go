package gitrepo

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yhlooo/motif/internal/content"
	"github.com/yhlooo/motif/internal/domain"
)

type fileStamp struct {
	Size          int64
	Mode          uint32
	Mtime, Ctime  int64
	Device, Inode uint64
}
type fileCache struct {
	Version           int
	Root, Fingerprint string
	ValidatedAt       int64
	Files             map[string]fileStamp
}

func stamp(info os.FileInfo) fileStamp {
	st := fileStamp{Size: info.Size(), Mode: uint32(info.Mode()), Mtime: info.ModTime().UnixNano()}
	systemStamp(info, &st)
	return st
}
func sortedPaths(paths map[string]bool) []string {
	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
func (r *Repository) cachePath() string {
	return filepath.Join(r.Objects.Dir, "capture-cache", content.Hash([]byte(r.GitDir))+".json")
}
func (r *Repository) loadFileCache(previous []domain.State, fingerprint string) fileCache {
	var c fileCache
	if len(previous) == 0 {
		return c
	}
	raw, e := os.ReadFile(r.cachePath())
	if e != nil || json.Unmarshal(raw, &c) != nil {
		return fileCache{}
	}
	if c.ValidatedAt > time.Now().UnixNano() || c.Version != 1 || c.Root != previous[0].Root() || c.Fingerprint != fingerprint {
		return fileCache{}
	}
	return c
}
func cachedFile(p string, info os.FileInfo, c fileCache, previous []domain.State) (domain.File, bool) {
	st := stamp(info)
	old, ok := c.Files[cacheKey(p)]
	// Refuse coarse/racy timestamps and platforms without reliable ctime/inode.
	safeBefore := c.ValidatedAt - int64(time.Second)
	if !ok || old != st || st.Ctime == 0 || st.Inode == 0 || st.Mtime >= safeBefore || st.Ctime >= safeBefore || len(previous) == 0 {
		return domain.File{}, false
	}
	f, ok := previous[0].Lookup(p)
	return f, ok && !f.OutOfScope && f.ContentRef != "" && (f.Mode == "100644" || f.Mode == "100755")
}
func (r *Repository) saveFileCache(c fileCache) {
	c.Version = 1
	raw, e := json.Marshal(c)
	if e != nil {
		return
	}
	path := r.cachePath()
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".cache-")
	if e != nil {
		return
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(raw); e != nil {
		f.Close()
		return
	}
	if f.Close() != nil {
		return
	}
	_ = os.Rename(f.Name(), path)
}

func cacheKey(path string) string { return base64.RawStdEncoding.EncodeToString([]byte(path)) }
