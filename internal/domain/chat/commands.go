package chat

import (
	"context"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "chats",
	Tool:    "Chat",
	Summary: "Open conversations and write to them.",
	Doc: `Conversations that persist.

A chat is the front door to execution, not a text box that forgets. Every
message is stored, every attempt to answer one records what it cost, and the
transcript of an autonomous run is the same structure in a different place.

## When to use
- To ask an agent something and have the exchange survive the session
- To bring a second agent into a thread by addressing it with @slug
- To read back what was said and what it cost

## When NOT to use
- Not to store what you learned — that is a memory
- Not to track work — that is a task`,
	Hint: `Address a specific agent with @slug. With no mention, a direct message goes to
the one agent in it, and anything else goes to the workspace orchestrator — the
result says which of the three happened, so a surprising answer is traceable to
the reason it was routed that way.`,
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "chats",
		Name:    "list",
		Aliases: []string{"ls"},
		Summary: "List conversations.",
		Doc: `List the conversations, most recently updated first.

The transcripts are left out: this answers what conversations exist, and
returning every message to answer it would be the most expensive call in the
system. Read one with ` + "`chats get`" + `.`,
		Examples: []command.Example{
			{Description: "the recent conversations", Input: ListInput{}},
			{Description: "only direct messages", Input: ListInput{Kind: KindDM}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List conversations", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Chat]{
		Group:   "chats",
		Name:    "get",
		Summary: "Read one conversation.",
		Doc: `Read a conversation with its whole transcript, including what each attempt
to answer cost in tokens.`,
		Examples: []command.Example{
			{Description: "read a conversation", Input: GetInput{Chat: "c-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a conversation", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Chat]{
		Group:   "chats",
		Name:    "create",
		Summary: "Open a conversation.",
		Doc: `Open a conversation.

Whoever opens it is in it. A private conversation its own creator could not read
is not something anyone means to make.`,
		Examples: []command.Example{
			{Description: "a workspace channel", Input: CreateInput{Title: "Planning"}},
			{
				Description: "a private thread with one agent",
				Input: CreateInput{
					Title: "Review", Kind: KindDM, Visibility: VisibilityPrivate,
					Participants: []Participant{{Type: ActorAgent, ID: "reviewer"}},
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Open a conversation"},
		Handler:     svc.Create,
	})

	command.MustRegister(reg, command.Command[SendInput, SendOutput]{
		Group:   "chats",
		Name:    "send",
		Summary: "Write to a conversation.",
		Doc: `Append a message and hand the turn to whoever should answer.

The message is persisted before anything is dispatched, so a runtime that dies
in between loses the answer and not the question. The result says which agent
was chosen and why, and whether a turn actually started — a message stored with
nobody to answer it is a real state, and one worth knowing about rather than
waiting through.`,
		Examples: []command.Example{
			{Description: "ask whoever is listening", Input: SendInput{Chat: "c-1", Text: "What changed in the gateway?"}},
			{Description: "ask a specific agent", Input: SendInput{Chat: "c-1", Text: "@reviewer take a look at this diff"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Send a message"},
		Handler:     svc.Send,
	})

	command.MustRegister(reg, command.Command[UpdateInput, *Chat]{
		Group:   "chats",
		Name:    "update",
		Summary: "Rename a conversation, or change who can read it.",
		Doc: `Change a conversation's name or its visibility.

Only what you name changes. There is no way to send a whole conversation back
and have it written, because that would let a rename drop the transcript or
reopen a private thread to the workspace without anybody asking for it.

Visibility has a consequence worth stating: ` + "`workspace`" + ` makes the
transcript readable by every member, and there is no undo for what somebody has
already read.`,
		Examples: []command.Example{
			{Description: "rename it", Input: UpdateInput{Chat: "c-1", Title: "Release planning"}},
			{Description: "open it to the workspace", Input: UpdateInput{Chat: "c-1", Visibility: VisibilityWorkspace}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Change a conversation", IdempotentHint: true},
		Handler:     svc.Update,
	})

	command.MustRegister(reg, command.Command[ClearInput, ClearOutput]{
		Group:   "chats",
		Name:    "clear",
		Summary: "Empty a conversation, keeping the conversation.",
		Doc: `Throw away what was said, and keep the room.

This is the "start over" of a long thread: the conversation, its participants
and its bindings survive; the transcript does not. It is separate from
` + "`chats update`" + ` on purpose — a rename that could drop a transcript by
carrying one field too many is a rename nobody can use safely.

Clearing one that is already empty is not an error. You asked for a state, and
that state holds.`,
		Examples: []command.Example{
			{Description: "start the thread over", Input: ClearInput{Chat: "c-1"}},
		},
		Registry: true,
		Annotations: command.Annotations{
			Title: "Clear a conversation", DestructiveHint: true, IdempotentHint: true,
		},
		Handler: svc.Clear,
	})

	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "chats",
		Name:    "delete",
		Aliases: []string{"rm"},
		Summary: "Remove a conversation and its transcript.",
		Doc: `Delete a conversation. The messages go with it.

The conversation has to exist. Reporting success for an identifier this
workspace has never had would hide the case that actually happens — a caller
deleting against the wrong workspace — behind an answer that reads like it
worked.`,
		Examples: []command.Example{
			{Description: "throw away a scratch thread", Input: DeleteInput{Chat: "c-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a conversation", DestructiveHint: true},
		Handler:     svc.Delete,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)     = (*Service)(nil).List
	_ func(context.Context, SendInput) (SendOutput, error)     = (*Service)(nil).Send
	_ func(context.Context, UpdateInput) (*Chat, error)        = (*Service)(nil).Update
	_ func(context.Context, ClearInput) (ClearOutput, error)   = (*Service)(nil).Clear
	_ func(context.Context, DeleteInput) (DeleteOutput, error) = (*Service)(nil).Delete
)
