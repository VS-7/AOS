package comment

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "comments",
	Tool:    "Comments",
	Summary: "The discussion inside a task.",
	Doc: `Progress, questions and feedback on a task.

When you are executing a task autonomously, nobody is watching the chat. The
comment is the durable, findable record of what you did and what you decided —
it is where the work is reported, not the conversation.

## Commands
- **list** — read a task's discussion, flat and grouped into threads
- **get** — read one comment
- **create** — write a comment, or reply to one
- **update** — rewrite something you wrote
- **delete** — remove something you wrote

## When to use
- **While executing a task:** post progress here, not in chat
- **When blocked:** say what is blocking and what you need
- **On a decision:** record the choice and the reason where the work is

## When NOT to use
- Not for the plan — that is a todo
- Not for what you learned in general — that is a memory`,
	Hint: `Authorship is server-side. You cannot set an author, and you can only edit or
delete what you wrote yourself.

Reply in-thread rather than starting a new top-level comment for every update.
A task with thirty top-level comments is a task nobody can read.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "comments",
		Name:    "list",
		Summary: "Read a task's discussion.",
		Doc: `Every comment on a task, oldest first, and the same comments grouped
into threads.

Both shapes are returned because both are needed: the flat list is the order
things were said, and the threads are how the discussion reads.`,
		Examples: []command.Example{
			{Description: "catch up on a task", Input: ListInput{Task: "t-42"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a task discussion", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Comment]{
		Group:   "comments",
		Name:    "get",
		Summary: "Read one comment.",
		Doc:     "Read one comment in full, with its author and the thread it belongs to.",
		Examples: []command.Example{
			{Description: "read a comment by identifier", Input: GetInput{Task: "t-42", ID: "c-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read one comment", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Comment]{
		Group:   "comments",
		Name:    "create",
		Aliases: []string{"add"},
		Summary: "Write a comment on a task.",
		Doc: `Post to a task's discussion.

You are the author because of who is making the call; there is no field for it.
Pass a parent to reply — and a reply to a reply joins the same thread rather
than nesting deeper.`,
		Examples: []command.Example{
			{Description: "report progress while executing", Input: CreateInput{
				Task: "t-42",
				Body: "Reproduced the failure: the denial pattern uses a path glob, so `*` stops at the separator and never matches a command line.",
			}},
			{Description: "answer in a thread", Input: CreateInput{
				Task: "t-42", Parent: "c-1", Body: "Fixed by matching the line with a spanning wildcard. Test added.",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Write a comment"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Comment]{
		Group:   "comments",
		Name:    "update",
		Summary: "Rewrite a comment you wrote.",
		Doc: `Change the body of your own comment.

Somebody else's comment is not yours to edit, and the attempt is refused. Reply
to it instead.`,
		Examples: []command.Example{
			{Description: "correct your own comment", Input: UpdateInput{
				Task: "t-42", ID: "c-1", Body: "Corrected: the pattern matches, but only when the command has no path in it.",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Rewrite a comment"},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "comments",
		Name:    "delete",
		Summary: "Remove a comment you wrote.",
		Doc: `Remove your own comment.

Its replies survive, promoted to the top level: deleting the message somebody
was answering must not delete their answer.`,
		Examples: []command.Example{
			{Description: "remove a comment posted by mistake", Input: DeleteInput{Task: "t-42", ID: "c-9"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Remove a comment", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)     = (*Service)(nil).List
	_ func(context.Context, DeleteInput) (DeleteOutput, error) = (*Service)(nil).Delete
)
