package skill

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/core/collections"
)

// Get reads one installed skill.
func (i *Installer) Get(ctx context.Context, id string) (*Skill, error) {
	id = strings.TrimSpace(id)
	found, err := i.repo.Get(ctx, collections.Key{"id": id})
	if err != nil {
		return nil, errNotFound(id)
	}
	out := found.Clone()
	return &out, nil
}

// ListOutput is every installed skill.
type ListOutput struct {
	Skills []Skill `json:"skills"`
	Total  int     `json:"total"`
}

// List returns every installed skill.
func (i *Installer) List(ctx context.Context) (ListOutput, error) {
	found, err := i.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, err
	}
	out := make([]Skill, len(found))
	for idx := range found {
		out[idx] = found[idx].Clone()
	}
	return ListOutput{Skills: out, Total: len(out)}, nil
}

// Views exposes the view domain's read surface a skill's own views live
// behind. It exists so a caller that already holds an *Installer — Task 11's
// delivery test is exactly this caller — does not also need a *view.Service
// of its own just to check what a skill's view rendered.
func (i *Installer) Views() Views { return i.views }
