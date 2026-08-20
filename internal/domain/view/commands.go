package view

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "views",
	Tool:    "Views",
	Summary: "Screens an agent composes over a collection, with no build and no deploy.",
	Doc: `A view is a tree of catalog components, bound to a collection's data, stored
as data — the frontend renders it the moment it is written.

Every button a view offers names a real command in the registry, checked when
the view is written and again when the button is pressed: a view is a
description of a screen, not a second way to write data that skips the
validation and the authorisation every other caller gets.

## Commands
- **list** — every view declared in the workspace, native and dynamic
- **get** — one declaration, including a skill-scoped one (pass skill)
- **render** — a view with its source data attached, ready to paint
- **execute-action** — press a button: dispatch the command it names
- **delete** — remove a view

## When to use
- **After a collection exists and needs a screen:** scaffold one, or compose
  a tree by hand with create
- **A skill-scoped view:** list returns it with its Skill field; get, render
  and delete need that field back to resolve it

## When NOT to use
- Not to run logic — every action is a command call, never code`,
	Hint: `execute-action can invoke anything the view's own tree names, including a
command that writes or deletes. Approve it the way you would approve calling
that command directly — the button is a shortcut to the call, not a lighter
version of it.`,
}

// ComponentsInput takes nothing — Components serves the same static catalog
// to every caller — but every command input still carries a reason.
type ComponentsInput struct {
	command.Reasoning
}

// DeleteOutput confirms what was removed. Service.Delete itself returns only
// an error; this is purely the shape the command surface answers with.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the view that was removed."`
}

// CreateRequest is the wire shape of views_create.
//
// Node is self-referential (Children []Node), and the JSON Schema library
// this system's schema generation runs on (jsonschema.For, called by
// command.Register) refuses outright any Go type with a cycle in it — there
// is no depth limit or $ref escape hatch, just a hard "cycle detected"
// error at registration, which would otherwise take the whole registry down
// at boot. Tree therefore arrives here as a raw JSON object instead of the
// typed Node CreateInput itself wants: the handler decodes it before
// calling Create, which still validates the same tree the same way no
// matter which shape it travelled the wire as.
type CreateRequest struct {
	ID          string          `json:"id" jsonschema:"Identifier for the view. Also its file name: lowercase, digits, hyphen and underscore only." validate:"required,notblank"`
	Name        string          `json:"name,omitempty" jsonschema:"Human name of the view. Example: \"Deals by stage\"."`
	Title       string          `json:"title,omitempty" jsonschema:"Heading the frontend shows above the rendered tree."`
	Description string          `json:"description,omitempty" jsonschema:"What this view is for."`
	Scope       string          `json:"scope,omitempty" jsonschema:"user or skill. Defaults to user."`
	Skill       string          `json:"skill,omitempty" jsonschema:"The skill this view ships with, when Scope is skill."`
	Source      Source          `json:"source" jsonschema:"Where this view's data comes from."`
	Tree        json.RawMessage `json:"tree" jsonschema:"The composed tree of catalog components: {component, props?, bind?, children?, actions?}, nested arbitrarily. Call views_components first to see what a component accepts."`

	command.Reasoning
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "views",
		Name:    "list",
		Summary: "List every declared view.",
		Doc:     "Every view declared in the workspace — native and dynamic, user-scoped and skill-scoped alike.",
		Examples: []command.Example{
			{Description: "everything declared", Input: ListInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List views", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *View]{
		Group:   "views",
		Name:    "get",
		Summary: "Read one view's declaration.",
		Doc: `Read one view's tree, unresolved — no source data attached. Use render to
get the tree together with the rows it binds to.

A skill-scoped view needs its Skill back to resolve — views_list reports it
on every entry.`,
		Examples: []command.Example{
			{Description: "a user's own view", Input: GetInput{ID: "contacts-table"}},
			{Description: "a view a skill brought", Input: GetInput{ID: "contacts-table", Skill: "crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a view", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	createHandler := func(ctx context.Context, in CreateRequest) (*View, error) {
		var tree Node
		if len(in.Tree) > 0 {
			if err := json.Unmarshal(in.Tree, &tree); err != nil {
				return nil, errTreeInvalid(err)
			}
		}
		return svc.Create(ctx, CreateInput{
			ID: in.ID, Name: in.Name, Title: in.Title, Description: in.Description,
			Scope: in.Scope, Skill: in.Skill, Source: in.Source, Tree: tree,
		})
	}
	command.MustRegister(reg, command.Command[CreateRequest, *View]{
		Group:   "views",
		Name:    "create",
		Summary: "Compose a new view.",
		Doc: `Declare a screen: a tree of catalog components bound to a collection's
fields.

The whole tree is validated against the source collection and against the
command registry before anything is written — an unknown component, an unbound
field or an action naming a command that does not exist is refused here, not
discovered later as a blank screen.`,
		Examples: []command.Example{
			{Description: "a scaffolded table, composed by hand", Input: CreateRequest{
				ID: "contacts-table", Title: "Contacts",
				Source: Source{Collection: "contacts"},
				Tree:   json.RawMessage(`{"component":"Table","bind":{"rows":"contacts"}}`),
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Compose a view"},
		Handler:     createHandler,
	})

	deleteHandler := func(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
		id := strings.TrimSpace(in.ID)
		if err := svc.Delete(ctx, in); err != nil {
			return DeleteOutput{}, err
		}
		return DeleteOutput{ID: id}, nil
	}
	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "views",
		Name:    "delete",
		Summary: "Remove a view.",
		Doc: `Remove a view's declaration.

A skill-scoped view is normally removed by uninstalling the skill, not by
calling this directly while the skill is still installed.`,
		Examples: []command.Example{
			{Description: "remove a view", Input: DeleteInput{ID: "contacts-table"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a view", DestructiveHint: true},
		Handler:     deleteHandler,
	})

	command.MustRegister(reg, command.Command[RenderInput, *Rendered]{
		Group:   "views",
		Name:    "render",
		Summary: "Resolve a view against its source data.",
		Doc: `The view's tree, with the rows its source names actually attached — what
the frontend paints. No build, no deploy: this is called fresh every time the
screen opens.`,
		Examples: []command.Example{
			{Description: "render a table before showing it", Input: RenderInput{ID: "contacts-table"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Render a view", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Render,
	})

	command.MustRegister(reg, command.Command[ExecuteActionInput, json.RawMessage]{
		Group:   "views",
		Name:    "execute-action",
		Summary: "Press a button in a view.",
		Doc: `Dispatch the command one of a view's declared actions names, by its
Label.

This does not execute anything itself: it resolves the action the view
already declared and validated, merges the caller's input over the action's
own, and invokes the named command through the same registry a CLI or an MCP
call would go through — the same input validation, the same authorisation.
What this can do therefore depends entirely on what the button names: a
read, or a delete, or anything registered.`,
		Examples: []command.Example{
			{Description: "click a row's delete button", Input: ExecuteActionInput{
				ID: "contacts-table", Label: "Delete", Input: map[string]any{"id": "c-1"},
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Run a view's action", DestructiveHint: true},
		Handler:     svc.ExecuteAction,
	})

	componentsHandler := func(ctx context.Context, _ ComponentsInput) ([]ComponentSpec, error) {
		return svc.Components(ctx)
	}
	command.MustRegister(reg, command.Command[ComponentsInput, []ComponentSpec]{
		Group:   "views",
		Name:    "components",
		Summary: "List the catalog of components a view can compose.",
		Doc: `Every component the design system publishes: its name, its declared
props and whether it accepts children — read this before composing a tree by
hand, so create is not a guess.`,
		Examples: []command.Example{
			{Description: "before composing a view", Input: ComponentsInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List components", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     componentsHandler,
	})

	command.MustRegister(reg, command.Command[ScaffoldInput, *View]{
		Group:   "views",
		Name:    "scaffold",
		Summary: "Compose a view an agent did not have to design by hand.",
		Doc: `Map a collection's declared fields to the component that shows each one,
producing a tree that already survives what create would validate — this
does not write anything, it only composes; call create with the result to
save it.`,
		Examples: []command.Example{
			{Description: "a table over a collection", Input: ScaffoldInput{Collection: "contacts", Kind: KindTable}},
			{Description: "a detail screen", Input: ScaffoldInput{Collection: "contacts", Kind: KindDetail}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Scaffold a view", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Scaffold,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)          = (*Service)(nil).List
	_ func(context.Context, GetInput) (*View, error)                = (*Service)(nil).Get
	_ func(context.Context, CreateInput) (*View, error)             = (*Service)(nil).Create
	_ func(context.Context, DeleteInput) error                      = (*Service)(nil).Delete
	_ func(context.Context, RenderInput) (*Rendered, error)         = (*Service)(nil).Render
	_ func(context.Context, ScaffoldInput) (*View, error)           = (*Service)(nil).Scaffold
	_ func(context.Context, ExecuteActionInput) (json.RawMessage, error) = (*Service)(nil).ExecuteAction
)
