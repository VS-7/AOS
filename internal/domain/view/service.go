package view

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/core/command"
	"github.com/OWNER/aos/internal/domain/collection"
)

// Service is the view aggregate: composition (Scaffold), validation
// (enforced inside Create, never deferred to render), persistence (List, Get,
// Create, Delete), rendering (Render) and dispatch (ExecuteAction).
type Service struct {
	repo        Repository
	collections Collections
	commands    Commands
	clock       Clock
}

// Deps is what the service is built from.
type Deps struct {
	Repo        Repository
	Collections Collections
	Commands    Commands
	Clock       Clock
}

// NewService wires the service over its ports.
func NewService(d Deps) *Service {
	return &Service{repo: d.Repo, collections: d.Collections, commands: d.Commands, clock: d.Clock}
}

// Components returns every component the design system publishes: the
// introspection an agent calls before composing a screen, over the same
// catalog frontend/scripts/gen-components.mjs generated for this task.
func (*Service) Components(context.Context) ([]ComponentSpec, error) {
	return Catalog(), nil
}

// ListInput selects the views List returns. It has no filter today: an
// agent composing screens lists what exists before naming a new one, and a
// workspace declares few enough views that listing all of them is never a
// cost worth guarding — the same call Collection.List already makes.
type ListInput struct {
	command.Reasoning
}

// ListOutput is every view declared in the workspace.
type ListOutput struct {
	Views []View `json:"views" jsonschema:"Every view declared in the workspace, including what a skill brought."`
	Total int    `json:"total" jsonschema:"How many there are."`
}

// GetInput names one view.
//
// Skill is optional and only needed for a skill-scoped view: the engine
// stores that declaration under a second, skill-qualified pattern
// (.aos/skills/{skill}/views/{id}.view.json), and {"id": id} alone does not
// resolve it. A record List returns already carries its own Skill
// (View.Skill), so a caller that got an id from List has what it needs to
// hand back here.
type GetInput struct {
	ID    string `json:"id" jsonschema:"Identifier of the view." validate:"required,notblank"`
	Skill string `json:"skill,omitempty" jsonschema:"The skill this view ships with, when it is skill-scoped. Required to resolve one — views_list reports it on every entry."`

	command.Reasoning
}

// CreateInput composes a new view. Validate runs against it before anything
// is written — see Create.
type CreateInput struct {
	ID          string `json:"id" jsonschema:"Identifier for the view. Also its file name: lowercase, digits, hyphen and underscore only." validate:"required,notblank"`
	Name        string `json:"name,omitempty" jsonschema:"Human name of the view. Example: \"Deals by stage\"."`
	Title       string `json:"title,omitempty" jsonschema:"Heading the frontend shows above the rendered tree."`
	Description string `json:"description,omitempty" jsonschema:"What this view is for."`
	Scope       string `json:"scope,omitempty" jsonschema:"user or skill. Defaults to user."`
	Skill       string `json:"skill,omitempty" jsonschema:"The skill this view ships with, when Scope is skill."`
	Source      Source `json:"source" jsonschema:"Where this view's data comes from."`
	Tree        Node   `json:"tree" jsonschema:"The composed tree of catalog components this view renders."`

	command.Reasoning
}

// DeleteInput names the view to remove. Skill is the same optional
// qualifier GetInput takes, for the same reason.
type DeleteInput struct {
	ID    string `json:"id" jsonschema:"Identifier of the view to remove." validate:"required,notblank"`
	Skill string `json:"skill,omitempty" jsonschema:"The skill this view ships with, when it is skill-scoped."`

	command.Reasoning
}

// RenderInput names the view to resolve against its source data. Skill is
// the same optional qualifier GetInput takes, for the same reason.
type RenderInput struct {
	ID    string `json:"id" jsonschema:"Identifier of the view to render." validate:"required,notblank"`
	Skill string `json:"skill,omitempty" jsonschema:"The skill this view ships with, when it is skill-scoped."`

	command.Reasoning
}

// Rendered is a view with its source data attached — what the frontend's
// renderer consumes to paint the screen, with no build and no deploy.
type Rendered struct {
	View       View                `json:"view"`
	Records    []collection.Record `json:"records"`
	RenderedAt time.Time           `json:"renderedAt"`
}

// ScaffoldInput names the collection and the shape of screen to compose one
// for.
type ScaffoldInput struct {
	Collection string `json:"collection" jsonschema:"Id of the collection to compose a screen for." validate:"required,notblank"`
	Kind       Kind   `json:"kind,omitempty" jsonschema:"table, board or detail. Defaults to table."`

	command.Reasoning
}

// ExecuteActionInput names the view and the action within it (by Label, not
// by index — an index shifts when the tree is edited, a label does not).
// Input is merged over the action's own declared Input, so a caller supplies
// only what the button's own declaration did not already fix, such as which
// record was clicked.
type ExecuteActionInput struct {
	ID    string         `json:"id" jsonschema:"Identifier of the view the action belongs to." validate:"required,notblank"`
	Label string         `json:"label" jsonschema:"Label of the action within the view's tree, as declared." validate:"required,notblank"`
	Input map[string]any `json:"input,omitempty" jsonschema:"Arguments to invoke the command with, merged over the action's own declared input."`

	command.Reasoning
}

// List returns every declared view.
func (s *Service) List(ctx context.Context, _ ListInput) (ListOutput, error) {
	found, err := s.repo.List(ctx, collections.Query{})
	if err != nil {
		return ListOutput{}, err
	}
	out := make([]View, len(found))
	for i := range found {
		out[i] = cloneView(found[i])
	}
	return ListOutput{Views: out, Total: len(out)}, nil
}

// Get reads one view.
func (s *Service) Get(ctx context.Context, in GetInput) (*View, error) {
	id := strings.TrimSpace(in.ID)
	v, err := s.repo.Get(ctx, keyOf(id, in.Skill))
	if err != nil {
		return nil, errNotFound(id)
	}
	out := cloneView(*v)
	return &out, nil
}

// keyOf builds the lookup key Get and Delete share — see the mirror of this
// in internal/domain/collection/service.go, which this is the same fix for.
func keyOf(id, skill string) collections.Key {
	key := collections.Key{"id": id}
	if skill = strings.TrimSpace(skill); skill != "" {
		key["skill"] = skill
	}
	return key
}

// Create validates the whole tree against the catalog and the source
// collection before writing anything.
//
// The order is the domain's reason to exist: resolve the source collection,
// validate the tree against it and against the command registry, and only
// then persist. An invalid view is refused here, naming what is wrong and
// where — it is never written, so it can never be the thing an agent opens
// later to find a blank screen.
func (s *Service) Create(ctx context.Context, in CreateInput) (*View, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, errIDRequired()
	}

	c, err := s.collections.Get(ctx, in.Source.Collection, in.Skill)
	if err != nil {
		return nil, err
	}

	scope := strings.TrimSpace(in.Scope)
	if scope == "" {
		scope = "user"
	}

	now := s.clock.Now()
	v := View{
		ID:          id,
		Name:        in.Name,
		Title:       in.Title,
		Description: in.Description,
		Scope:       scope,
		Skill:       in.Skill,
		Source:      in.Source,
		Tree:        in.Tree,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := Validate(v, *c, s.commands); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, &v); err != nil {
		return nil, err
	}
	out := cloneView(v)
	return &out, nil
}

// Delete removes a view. It reads first so a missing id is refused naming
// what was asked for, rather than surfacing whatever a repository's own
// Delete happens to report for a key that was never there.
func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	id := strings.TrimSpace(in.ID)
	if _, err := s.Get(ctx, GetInput{ID: id, Skill: in.Skill}); err != nil {
		return err
	}
	return s.repo.Delete(ctx, keyOf(id, in.Skill))
}

// Render resolves a view's source and returns the tree with the data
// attached — what the frontend's renderer consumes. Only the first Sort
// entry orders the query: collection.RecordQuery orders by one field, and a
// view that needs more than one is asking for more than this render path
// gives it today.
func (s *Service) Render(ctx context.Context, in RenderInput) (*Rendered, error) {
	v, err := s.Get(ctx, GetInput(in))
	if err != nil {
		return nil, err
	}

	q := collection.RecordQuery{
		Filters: v.Source.Filter,
		Limit:   v.Source.Limit,
	}
	if len(v.Source.Sort) > 0 {
		q.OrderBy = v.Source.Sort[0].Field
		q.Desc = v.Source.Sort[0].Desc
	}

	records, err := s.collections.ListRecords(ctx, v.Source.Collection, v.Skill, q)
	if err != nil {
		return nil, err
	}

	return &Rendered{View: *v, Records: cloneRecords(records), RenderedAt: s.clock.Now()}, nil
}

// Scaffold composes a view an agent did not have to design by hand: it maps
// each declared field of a collection to the component that shows it, so
// what Create receives already survives its own validation. See scaffold.go.
func (s *Service) Scaffold(ctx context.Context, in ScaffoldInput) (*View, error) {
	id := strings.TrimSpace(in.Collection)
	c, err := s.collections.Get(ctx, id, "")
	if err != nil {
		return nil, err
	}

	kind := in.Kind
	if kind == "" {
		kind = KindTable
	}

	return &View{
		ID:     id + "-" + string(kind),
		Name:   c.Name,
		Title:  c.Name,
		Scope:  "user",
		Source: Source{Collection: c.ID},
		Tree:   scaffoldTree(*c, kind),
	}, nil
}

// ExecuteAction does not execute anything itself: it resolves the action the
// view itself declared and dispatches through the command registry. Naming a
// real command in ExecuteActionInput is not enough, and naming a Label the
// view never declared is refused before the registry is ever consulted —
// otherwise a view would be a way to call anything in it from a button
// nobody wrote into the tree.
func (s *Service) ExecuteAction(ctx context.Context, in ExecuteActionInput) (json.RawMessage, error) {
	v, err := s.Get(ctx, GetInput{ID: in.ID})
	if err != nil {
		return nil, err
	}

	action, ok := findAction(v.Tree, in.Label)
	if !ok {
		return nil, errActionNotDeclared(in.ID, in.Label)
	}
	// Re-checked here, not only at Create: a command can be unregistered
	// (a skill removed, a build that dropped a tool) after the view that
	// names it was written and validated.
	if !s.commands.Has(action.Command) {
		return nil, errActionCommandUnknown("tree", action.Command)
	}

	merged := make(map[string]any, len(action.Input)+len(in.Input))
	for k, val := range action.Input {
		merged[k] = val
	}
	for k, val := range in.Input {
		merged[k] = val
	}

	payload, err := json.Marshal(merged)
	if err != nil {
		return nil, errActionInputInvalid(in.ID, in.Label, err)
	}
	return s.commands.Invoke(ctx, action.Command, payload)
}

// findAction searches a tree for the action with the given label, depth
// first. Actions are looked up by label rather than by position because a
// tree is edited between the moment a view is written and the moment a
// button on it is clicked, and an index into Children would silently point
// at a different node once it is.
func findAction(n Node, label string) (Action, bool) {
	for _, a := range n.Actions {
		if a.Label == label {
			return a, true
		}
	}
	for _, child := range n.Children {
		if a, ok := findAction(child, label); ok {
			return a, true
		}
	}
	return Action{}, false
}

// cloneView deep-copies every reference-typed field a caller could reach
// through a returned View, so mutating what List, Get or Create hand back
// cannot corrupt whatever a Repository implementation is holding underneath
// it — an in-memory index, in particular, may be handing back the exact map
// instance it indexes by.
func cloneView(v View) View {
	v.Source = cloneSource(v.Source)
	v.Tree = cloneNode(v.Tree)
	return v
}

func cloneSource(s Source) Source {
	if s.Filter != nil {
		s.Filter, _ = collections.CloneJSON(s.Filter).(map[string]any)
	}
	if s.Sort != nil {
		s.Sort = append([]SortSpec(nil), s.Sort...)
	}
	return s
}

func cloneNode(n Node) Node {
	if n.Props != nil {
		n.Props, _ = collections.CloneJSON(n.Props).(map[string]any)
	}
	if n.Bind != nil {
		bind := make(map[string]string, len(n.Bind))
		for k, val := range n.Bind {
			bind[k] = val
		}
		n.Bind = bind
	}
	if n.Children != nil {
		children := make([]Node, len(n.Children))
		for i, child := range n.Children {
			children[i] = cloneNode(child)
		}
		n.Children = children
	}
	if n.Actions != nil {
		actions := make([]Action, len(n.Actions))
		for i, a := range n.Actions {
			actions[i] = cloneAction(a)
		}
		n.Actions = actions
	}
	return n
}

func cloneAction(a Action) Action {
	if a.Input != nil {
		a.Input, _ = collections.CloneJSON(a.Input).(map[string]any)
	}
	return a
}

// cloneRecords deep-copies the Data map of every record Render hands out. The
// Collections port is another domain's boundary, not this package's own
// cache — but a fake or a future adapter that skips its own copy would
// otherwise let a caller of Render corrupt state this package does not own
// and has no way to know was shared.
func cloneRecords(recs []collection.Record) []collection.Record {
	out := make([]collection.Record, len(recs))
	for i, r := range recs {
		if r.Data != nil {
			r.Data, _ = collections.CloneJSON(r.Data).(map[string]any)
		}
		out[i] = r
	}
	return out
}
