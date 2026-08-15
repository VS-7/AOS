package fscollections

import (
	"strings"
	"sync"

	"github.com/OWNER/aos/internal/core/collections"
)

// memIndex keeps the decoded front matter of every record so that a filtered
// List does not re-parse the filesystem. Bodies are not cached: they can be
// large and are only needed on Get.
//
// The index is a cache, never a source of truth. Every entry carries the
// version of the file it came from and is discarded the moment the file on disk
// differs — which is what makes an edit made outside the system, in an editor
// or by a git checkout, invisible to nobody.
type memIndex struct {
	mu      sync.RWMutex
	entries map[string]indexEntry
}

type indexEntry struct {
	version collections.Version
	record  any
}

func newMemIndex() *memIndex {
	return &memIndex{entries: map[string]indexEntry{}}
}

// NewIndex builds an index that several repositories of one workspace share.
func NewIndex() *memIndex { return newMemIndex() }

func (i *memIndex) get(rel string, version collections.Version) (any, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	e, ok := i.entries[rel]
	if !ok || !e.version.Equal(version) {
		return nil, false
	}
	return e.record, true
}

func (i *memIndex) put(rel string, version collections.Version, record any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries[rel] = indexEntry{version: version, record: record}
}

func (i *memIndex) invalidate(rel string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.entries, rel)
}

// invalidatePrefix drops a whole subtree, which is what a cascade delete needs:
// removing a task directory invalidates its todos, comments and runs at once.
func (i *memIndex) invalidatePrefix(rel string) {
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
func (i *memIndex) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

// Clear empties the index. `aos workspace reindex` and the recovery path after
// an unattributable change use it.
func (i *memIndex) Clear() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries = map[string]indexEntry{}
}
