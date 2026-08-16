package theme

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "themes",
	Tool:    "Themes",
	Summary: "The appearance of the application.",
	Doc: `The colour presets the interface can wear.

A theme is four colours and a dial — a surface, an ink, an accent, and how
strongly the layers between them separate. Everything else is derived: the
cards, the borders, the chart series, and the colour of every memory category
in the graph.

## Commands
- **list** — the themes available, built-in first
- **get** — one theme with the CSS properties it renders to
- **install** — add a preset of your own
- **delete** — remove one of yours

## When to use
- **Someone asks to change how the app looks:** list, then set it in config
- **Someone brings a palette:** install it as a preset

## When NOT to use
- Not to change one colour in one screen — that is a component, not a theme`,
	Hint: `Every colour the interface uses comes from a token, and every token is derived
from the four colours a theme declares. That is why a theme is short, and why a
palette that renders an unreadable interface is caught when it is installed
rather than when somebody squints at it.

A theme with both a light and a dark palette can follow the system. One with a
single palette cannot honour a preference it has no colours for.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "themes",
		Name:    "list",
		Summary: "List the themes available.",
		Doc:     "The presets this build ships with, followed by the ones installed here.",
		Examples: []command.Example{
			{Description: "everything available", Input: ListInput{}},
			{Description: "only what can go light", Input: ListInput{Appearance: Light}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List themes", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, GetOutput]{
		Group:   "themes",
		Name:    "get",
		Summary: "Read one theme and what it renders to.",
		Doc: `One theme: the palette its author wrote, and the CSS custom properties
that palette produces for each appearance it offers.

The rendered properties are what the interface actually sets on the document
root. Reading them is how you find out what a theme will look like without
switching to it.`,
		Examples: []command.Example{
			{Description: "read a theme", Input: GetInput{ID: "nord"}},
			{Description: "as the desktop window would render it", Input: GetInput{ID: "nord", Native: true}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one theme", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[InstallInput, *Theme]{
		Group:   "themes",
		Name:    "install",
		Summary: "Add a theme of your own.",
		Doc: `Install a preset.

It is rendered and checked before it is stored: a palette that produces fewer
properties than the interface reads is refused here rather than discovered later
as an element nobody can see.`,
		Examples: []command.Example{
			{
				Description: "a dark preset",
				Input: InstallInput{
					ID: "midnight", Name: "Midnight",
					Variants: map[Appearance]Palette{
						Dark: {Surface: "#0b0d12", Ink: "#e6e9ef", Accent: "#7aa2f7", Contrast: 70},
					},
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Install a theme"},
		Handler:     svc.Install,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "themes",
		Name:    "delete",
		Summary: "Remove a theme you installed.",
		Doc: `Delete one of your presets.

A built-in theme cannot be removed: it lives in the binary, so deleting it would
only mean pretending it is not there.`,
		Examples: []command.Example{
			{Description: "remove a preset", Input: DeleteInput{ID: "midnight"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a theme", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error) = (*Service)(nil).List
	_ func(context.Context, GetInput) (GetOutput, error)   = (*Service)(nil).Get
)
