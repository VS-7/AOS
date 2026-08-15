package fscollections

import (
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/core/collections"
)

// Index keeps the decoded front matter of every record so that a filtered
// List does not re-parse the filesystem. Bodies are not cached: they can be
// large and are only needed on Get.
//
// The index is a cache, never a source of truth. Every entry carries the
// version of the file it came from and is discarded the moment the file on disk
// differs — which is what makes an edit made outside the system, in an editor
// or by a git checkout, invisible to nobody.
type Index struct {
	mu      sync.RWMutex
	entries map[string]indexEntry
}

type indexEntry struct {
	version collections.Version
	record  any
}

// NewIndex builds an index that several repositories of one workspace share.
func NewIndex() *Index { return &Index{entries: map[string]indexEntry{}} }

func (i *Index) get(rel string, version collections.Version) (any, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	e, ok := i.entries[rel]
	if !ok || !e.version.Equal(version) {
		return nil, false
	}
	return e.record, true
}

func (i *Index) put(rel string, version collections.Version, record any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries[rel] = indexEntry{version: version, record: record}
}

func (i *Index) invalidate(rel string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.entries, rel)
}

// invalidatePrefix drops a whole subtree, which is what a cascade delete needs:
// removing a task directory invalidates its todos, comments and runs at once.
func (i *Index) invalidatePrefix(rel string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.entries, rel)
	prefix := strings.TrimSuffix(rel, "/") + "/"
	for k := range i.entries {
		if strings.HasPrefix(k, prefix) {
			delete(i.entries, k)
		}
	}
}

// Len reports how many records are cached. Tests use it to prove the cache is
// actually being hit rather than silently bypassed.
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

// Clear empties the index. `aos workspace reindex` and the recovery path after
// an unattributable change use it.
func (i *Index) Clear() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries = map[string]indexEntry{}
}
