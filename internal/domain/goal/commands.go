package goal

import "github.com/OWNER/aos/internal/core/command"

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "goals",
	Tool:    "Goals",
	Summary: "Strategic outcomes that daily work aligns to.",
	Doc: `Before planning or executing significant work, check active goals to align
your efforts and avoid strategically useless results — technically correct
work that serves nothing.

## Commands
- **list** — goals matching a filter
- **get** — one goal in full
- **create** — a new goal
- **update** — change a goal's fields, including its status
- **delete** — remove a goal; every task that referenced it is disassociated,
  not removed

## When to use
- **Before starting significant work:** call goals_list with status active
  first, so the work chosen actually serves one of them

## When NOT to use
- Not for tracking a single unit of work — that is [[Task (Go)]]; a goal is
  the destination several tasks might serve`,
	Hint: `A goal without a measure is aspirational, not checkable. Set one when the
outcome can be verified, so later work can tell whether it was actually
served.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, []Goal]{
		Group:   "goals",
		Name:    "list",
		Summary: "List goals matching a filter.",
		Doc:     "Every goal matching the given status, project or text filter.",
		Examples: []command.Example{
			{Description: "everything active", Input: ListInput{Query: Query{Status: []Status{StatusActive}}}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List goals", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Goal]{
		Group:   "goals",
		Name:    "get",
		Summary: "Read one goal in full.",
		Doc:     "Read a goal: its status, its measure, and what it is for.",
		Examples: []command.Example{
			{Description: "read a goal before aligning work to it", Input: GetInput{ID: "launch-v1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a goal", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Goal]{
		Group:   "goals",
		Name:    "create",
		Summary: "Create a new goal.",
		Doc:     "Create a strategic outcome. Its id is derived from the title.",
		Examples: []command.Example{
			{Description: "a new goal", Input: CreateInput{Title: "Launch V1", Measure: "v1 released and announced"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a goal"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Goal]{
		Group:   "goals",
		Name:    "update",
		Summary: "Change a goal's fields.",
		Doc:     "Change the describable parts of a goal, including moving it to achieved or abandoned.",
		Examples: []command.Example{
			{Description: "mark a goal achieved", Input: UpdateInput{ID: "launch-v1", Status: statusPtr(StatusAchieved)}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a goal"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "goals",
		Name:    "delete",
		Summary: "Remove a goal.",
		Doc:     "Remove a goal. Every task that referenced it is disassociated, not deleted.",
		Examples: []command.Example{
			{Description: "remove an abandoned goal", Input: DeleteInput{ID: "launch-v1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a goal", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func statusPtr(s Status) *Status { return &s }
