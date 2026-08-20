package collection

import (
	"path"

	"github.com/OWNER/aos/internal/core/collections"
)

// DescriptorFor turns a declaration into the engine descriptor its records are
// stored under.
//
// The collection's id is baked into the pattern rather than left as a
// placeholder: a collection named "contacts" becomes a descriptor named
// "contacts" whose records live at a fixed path. That is what keeps Query.Key
// meaning what it means everywhere else — {id} identifies a record, not which
// collection it belongs to.
//
// A skill-scoped collection gets a second, read-only pattern, by the same rule
// the natives already use: a pattern with a wildcard is not writable.
func DescriptorFor(c Collection) (collections.Descriptor, error) {
	if err := validName(c.ID); err != nil {
		return collections.Descriptor{}, err
	}
	ext := "md"
	format := collections.FormatMarkdown
	if c.Format == FormatJSON {
		ext = "json"
		format = collections.FormatJSON
	}

	patterns := []*collections.Pattern{
		collections.MustCompile(path.Join(collections.Root, "collections", c.ID, "records", "{id}."+ext)),
	}
	if c.Scope == ScopeSkill {
		patterns = append(patterns, collections.MustCompile(
			path.Join(collections.Root, "skills", "*", "collections", c.ID, "records", "{id}."+ext)))
	}
	return collections.Descriptor{
		Name:          c.ID,
		Patterns:      patterns,
		Format:        format,
		CascadeDelete: false,
	}, nil
}

// validName keeps an id usable as a path segment. It is the same reasoning as
// slugify's: an id is a directory name, and a directory name with a separator
// or a dot-dot in it is a path traversal.
func validName(id string) error {
	if id == "" || id == "." || id == ".." {
		return errNameInvalid(id)
	}
	for _, r := range id {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return errNameInvalid(id)
		}
	}
	return nil
}
