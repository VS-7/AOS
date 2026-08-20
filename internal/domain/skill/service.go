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
	Skills []Skill `json:"skills" jsonschema:"Every skill installed in this workspace."`
	Total  int     `json:"total" jsonschema:"How many there are."`
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

// Update turns a skill's live behaviour on or off without removing it.
//
// Active is the only field this touches — it is the only one the ported
// frontend ever writes (a toggle, not a form), and it is deliberately the
// only one this domain lets change in place: content, permissions and
// inventory are what Install verified and a person consented to, and editing
// any of them here would let a later write bypass that consent entirely.
// Changing any of those goes through Uninstall and a fresh Install.
//
// It does not itself stop a disabled skill's hooks from intercepting or its
// toolsets from connecting — that enforcement is Hooks' and Toolsets' own,
// reading Active at the point they act, not this method's to reach into.
func (i *Installer) Update(ctx context.Context, in UpdateInput) (*Skill, error) {
	id := strings.TrimSpace(in.ID)
	current, err := i.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Active != nil {
		current.Active = *in.Active
	}
	current.UpdatedAt = i.clock.Now()

	toWrite := current.Clone()
	if err := i.repo.Update(ctx, &toWrite, collections.Version{}); err != nil {
		return nil, errUpdateFailed(id, err)
	}
	return current, nil
}
