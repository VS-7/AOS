package template

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "templates",
	Tool:    "Template",
	Summary: "Reusable Liquid generators for work with a repeatable shape — task briefs, plans, reports.",
	Doc: `A template combines a stable Liquid body with a declared set of variables.
Inspect a template's variables before rendering it, the same inspect-before-
execute pattern every composite tool in this system follows.

## Commands
- **list** — every template, optionally filtered by skill or a text query
- **get** — one template's full Liquid body and declared variables
- **create** — author a new template
- **update** — change an existing template's body, variables or metadata
- **delete** — remove a template
- **render** — run a template's body against variables, bounded by a
  timeout and an output size cap

## When to use
- **Work has a recognizable, repeatable shape:** a task brief, a plan, a
  report — list or get before writing one from scratch
- **Before render:** get the template first to learn what variables it
  expects; a missing required one refuses with that same instruction

## When NOT to use
- Not for one-off text with no reuse value — a template is worth the
  round-trip only when the shape repeats`,
	Hint: `render is the one place this system runs user-authored Liquid. It never sees
config, secrets or the process environment — only the variables given, plus
whatever the template's own declared defaults fill in.

A template that does not parse as Liquid is refused by create or update
before it is ever saved — it cannot become a render failure discovered
later.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "templates",
		Name:    "list",
		Summary: "List templates, optionally filtered by skill or text.",
		Doc:     "Every template matching the filter, without their Liquid content — get one by id to read its body.",
		Examples: []command.Example{
			{Description: "everything", Input: ListInput{}},
			{Description: "a skill's own templates", Input: ListInput{Skill: "igniter-js"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List templates", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Template]{
		Group:   "templates",
		Name:    "get",
		Summary: "Read one template's Liquid body and declared variables.",
		Doc:     "Read a template in full — call this before render to learn what variables it expects.",
		Examples: []command.Example{
			{Description: "inspect before rendering", Input: GetInput{ID: "plan"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a template", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Template]{
		Group:   "templates",
		Name:    "create",
		Summary: "Author a new template.",
		Doc:     "Create a template. Content is validated as Liquid before anything is written; a broken template is refused here, not discovered at render.",
		Examples: []command.Example{
			{Description: "a minimal template", Input: CreateInput{ID: "plan", Name: "plan", Content: "## {{ name }}"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a template"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Template]{
		Group:   "templates",
		Name:    "update",
		Summary: "Change an existing template.",
		Doc: `Change the describable parts of a template. A field left nil is
unchanged; Variables, given at all, replaces the field wholesale — there is
no per-entry merge. New Content, if given, is re-validated as Liquid before
anything is written.`,
		Examples: []command.Example{
			{Description: "change the body", Input: UpdateInput{ID: "plan", Content: stringPtr("## {{ name }}\n\n{{ summary }}")}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a template"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "templates",
		Name:    "delete",
		Summary: "Remove a template.",
		Doc:     "Remove a template. Idempotent: deleting what is already gone succeeds rather than erroring.",
		Examples: []command.Example{
			{Description: "remove a template", Input: DeleteInput{ID: "plan"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a template", DestructiveHint: true},
		Handler:     svc.Delete,
	})

	command.MustRegister(reg, command.Command[RenderInput, RenderResult]{
		Group:   "templates",
		Name:    "render",
		Summary: "Render a template's body against variables.",
		Doc: `Runs a template's Liquid body against the variables given, bounded by a
timeout and an output size cap. A required variable with no default and no
value given refuses the render, naming which one and pointing at
templates_get.`,
		Examples: []command.Example{
			{Description: "render with variables", Input: RenderInput{ID: "plan", Variables: map[string]any{"name": "auth"}}},
		},
		Registry: true,
		Annotations: command.Annotations{
			Title: "Render a template", ReadOnlyHint: true,
		},
		Handler: svc.Render,
	})
}

func stringPtr(s string) *string { return &s }

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)     = (*Service)(nil).List
	_ func(context.Context, GetInput) (*Template, error)       = (*Service)(nil).Get
	_ func(context.Context, CreateInput) (*Template, error)    = (*Service)(nil).Create
	_ func(context.Context, UpdateInput) (*Template, error)    = (*Service)(nil).Update
	_ func(context.Context, DeleteInput) (DeleteOutput, error) = (*Service)(nil).Delete
	_ func(context.Context, RenderInput) (RenderResult, error) = (*Service)(nil).Render
)
