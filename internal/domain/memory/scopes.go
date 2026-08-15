package memory

import "github.com/bmatcuk/doublestar/v4"

// ScopesMode decides what happens to a memory that has no scopes at all.
type ScopesMode string

const (
	// ScopesLax includes unscoped memories. It is the default, because a
	// memory without scopes is usually one that applies everywhere.
	ScopesLax ScopesMode = "lax"

	// ScopesStrict excludes them, which is what you want when the question is
	// specifically "what do I know about these files".
	ScopesStrict ScopesMode = "strict"
)

// matchScopes reports whether a memory applies under the given scope filter.
//
// The filter entries are glob patterns and the memory's scopes are the strings
// they are tested against — which are themselves usually globs, sometimes
// concrete paths. That is the original's direction and it is the useful one: a
// query for "src/**" finds memories anchored anywhere under src, whether they
// recorded a directory or a single file.
func matchScopes(m Memory, filters []string, mode ScopesMode) bool {
	if len(filters) == 0 {
		return true
	}
	if len(m.Scopes) == 0 {
		return mode != ScopesStrict
	}
	for _, pattern := range filters {
		for _, scope := range m.Scopes {
			// An invalid pattern matches nothing rather than failing the whole
			// recall: one malformed filter should not make the others useless.
			if ok, err := doublestar.Match(pattern, scope); err == nil && ok {
				return true
			}
		}
	}
	return false
}
