// Package project is the durable container for related work that spans
// multiple tasks and goals — a top-level organizing boundary under a
// workspace, not a lifecycle of its own.
package project

import "time"

// Status is where a project sits.
type Status string

const (
	Active   Status = "active"
	Paused   Status = "paused"
	Done     Status = "done"
	Archived Status = "archived"
)

// Statuses lists the four in no particular order — a project does not move
// through them on rails the way a Task does.
var Statuses = []Status{Active, Paused, Done, Archived}

// EnumValues publishes the project lifecycle to every schema.
func (s Status) EnumValues() []string {
	out := make([]string, 0, len(Statuses))
	for _, v := range Statuses {
		out = append(out, string(v))
	}
	return out
}

// Valid reports whether s is one of the four.
func (s Status) Valid() bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// Project is a durable container for related work: goals and tasks associate
// with it by id, and deleting it unlinks them rather than taking them with it
// — see Service.Delete.
type Project struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name"                   json:"name"`
	Description string `yaml:"description,omitempty"  json:"description,omitempty"`
	Status      Status `yaml:"status"                 json:"status"`
	Color       string `yaml:"color,omitempty"        json:"color,omitempty"`

	// Icon is a Lucide icon name, image URL or data URI, wherever the
	// project is listed — carried over from the original's schema, which
	// this design doc's own sketch omitted.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`

	// Source is the absolute host directory this project is bound to,
	// metadata only — never a symlink, never written to. Also carried over
	// from the original; validated on Create/Update by validateSource.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// Paths are the globs this project owns, matched with doublestar — the
	// same shape as Instruction's Paths and Memory's Scopes, so an agent can
	// infer the project from the file it is already working in.
	Paths []string `yaml:"paths,omitempty" json:"paths,omitempty"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"`
}

// Clone returns a deep copy: the slice fields never alias the receiver's,
// which is what makes it safe to hand a caller a value backed by the same
// record the repository still holds.
func (p Project) Clone() Project {
	c := p
	if p.Paths != nil {
		c.Paths = append([]string(nil), p.Paths...)
	}
	return c
}
