package project

import "github.com/OWNER/aos/internal/core/command"

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "projects",
	Tool:    "Projects",
	Summary: "Durable, top-level containers organizing related goals, tasks and files into coherent bodies of work.",
	Doc: `A project structures a long-running effort that spans multiple tasks —
associate a task or goal with the right project to keep work traceable. The
hierarchy workspace → project → {goal, task} is not rigid: a task can exist
with no project and no goal.

## Commands
- **list** — every project, optionally filtered by status
- **get** — one project's full record
- **create** — a new project
- **update** — change a project's describable fields
- **delete** — remove a project's own record without touching the tasks or
  goals that were organized under it

## When to use
- **Structuring a long-running effort:** create a project before starting a
  batch of related tasks, so they stay traceable as a group
- **Filtering by relevance:** list with a status filter before assuming a
  project is still active

## When NOT to use
- Not for a single, standalone task — a project is for work that spans
  several`,
	Hint: `Deleting a project never deletes the work done inside it: tasks and goals
are unlinked, not removed. paths lets an agent infer the right project from
the file it is already editing, without asking.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, []Project]{
		Group:   "projects",
		Name:    "list",
		Summary: "List projects, optionally filtered by status.",
		Doc:     "Every project in the workspace, or only those in one status.",
		Examples: []command.Example{
			{Description: "everything active", Input: ListInput{Query: Query{Status: Active}}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List projects", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Project]{
		Group:   "projects",
		Name:    "get",
		Summary: "Read one project's full record.",
		Doc:     "Read a project in full, including its paths and its bound host source, if any.",
		Examples: []command.Example{
			{Description: "read before updating", Input: GetInput{ID: "my-app"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a project", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Project]{
		Group:   "projects",
		Name:    "create",
		Summary: "Create a new project.",
		Doc:     "Start a new organizing boundary for related work. id defaults to a slug of name when omitted.",
		Examples: []command.Example{
			{Description: "a new project bound to a host directory", Input: CreateInput{Name: "My App", Source: "/Users/dev/projects/my-app"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a project"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Project]{
		Group:   "projects",
		Name:    "update",
		Summary: "Change a project's describable fields.",
		Doc:     "A field left nil is unchanged; paths, given at all, replaces the field wholesale.",
		Examples: []command.Example{
			{Description: "pause a project", Input: UpdateInput{ID: "my-app", Status: statusPtr(Paused)}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a project"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "projects",
		Name:    "delete",
		Summary: "Remove a project's own record.",
		Doc:     "Never cascades: tasks and goals under this project are unlinked, not removed.",
		Examples: []command.Example{
			{Description: "remove a finished project", Input: DeleteInput{ID: "my-app"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a project", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

func statusPtr(s Status) *Status { return &s }
