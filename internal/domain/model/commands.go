package model

import (
	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "models",
	Tool:    "Models",
	Summary: "Which models the connected providers actually serve.",
	Doc: `Ask each connected provider what it can serve, right now.

The answer comes from the provider, not from a list inside this build. A model
that appears here exists and this installation can authenticate to it; a model
that does not appear either is not served or is not reachable with the
credential configured.

## Commands
- **list** — every connected provider's catalogue, or one named provider's

## When to use
- **Before pointing an agent or a slot at a model:** so the id is one that exists
- **When a turn fails with an unknown model:** to see what the provider replaced it with

## When NOT to use
- Not to find out which providers are *connected* — that is the configuration`,
	Hint: `A provider that is connected but failing to answer comes back as an entry with
an empty catalogue and the reason in its error field, not as a missing entry and
not as a failed call. One expired key does not hide the other providers'
catalogues.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "models",
		Name:    "list",
		Summary: "List the models the connected providers serve.",
		Doc: `Ask every connected provider for its catalogue, or one named provider for its own.

Each provider is asked over the network with the credential this installation
has for it, so this is a real question with a real latency, not a lookup.`,
		Examples: []command.Example{
			{Description: "everything reachable", Input: ListInput{}},
			{Description: "one provider", Input: ListInput{Provider: "anthropic"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List models", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})
}
