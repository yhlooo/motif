package gitrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/yhlooo/motif/internal/domain"
	"sync"
)

// boundedCache stores disposable immutable results. Errors are never cached.
// Eviction clears the cache rather than retaining unbounded history metadata.
type boundedCache struct {
	mu     sync.Mutex
	values map[string][]byte
	size   int
}

func (c *boundedCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[key]
	return bytes.Clone(v), ok
}
func (c *boundedCache) put(key string, value []byte) {
	if len(value) > 4<<20 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.size+len(value) > 16<<20 || len(c.values) >= 256 {
		c.values = nil
		c.size = 0
	}
	if c.values == nil {
		c.values = map[string][]byte{}
	}
	c.size -= len(c.values[key])
	c.values[key] = bytes.Clone(value)
	c.size += len(value)
}
func (r *Repository) Diff(ctx context.Context, before, after domain.State) ([]FileDiff, error) {
	key := before.Root() + "/" + after.Root()
	if data, ok := r.diffCache.get(key); ok {
		var result []FileDiff
		if err := json.Unmarshal(data, &result); err == nil {
			return result, nil
		}
	}
	result, err := r.diffUncached(ctx, before, after)
	if err == nil {
		data, _ := json.Marshal(result)
		r.diffCache.put(key, data)
	}
	return result, err
}

var lineCache boundedCache

func MapLines(ctx context.Context, source, target []byte, start, end int) ([]int, error) {
	key := fmt.Sprintf("%x/%x/%d/%d", sha256.Sum256(source), sha256.Sum256(target), start, end)
	if data, ok := lineCache.get(key); ok {
		var result []int
		if err := json.Unmarshal(data, &result); err == nil {
			return result, nil
		}
	}
	result, err := mapLinesUncached(ctx, source, target, start, end)
	if err == nil {
		data, _ := json.Marshal(result)
		lineCache.put(key, data)
	}
	return result, err
}
