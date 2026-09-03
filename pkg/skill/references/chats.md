# Chats

Open conversations and write to them.

Conversations that persist.

A chat is the front door to execution, not a text box that forgets. Every
message is stored, every attempt to answer one records what it cost, and the
transcript of an autonomous run is the same structure in a different place.

## When to use
- To ask an agent something and have the exchange survive the session
- To bring a second agent into a thread by addressing it with @slug
- To read back what was said and what it cost

## When NOT to use
- Not to store what you learned — that is a memory
- Not to track work — that is a task

## Commands

### `chats_clear`

Empty a conversation, keeping the conversation.

Throw away what was said, and keep the room.

This is the "start over" of a long thread: the conversation, its participants
and its bindings survive; the transcript does not. It is separate from
`chats update` on purpose — a rename that could drop a transcript by
carrying one field too many is a rename nobody can use safely.

Clearing one that is already empty is not an error. You asked for a state, and
that state holds.

- start the thread over

### `chats_create`

Open a conversation.

Open a conversation.

Whoever opens it is in it. A private conversation its own creator could not read
is not something anyone means to make.

- a workspace channel
- a private thread with one agent

### `chats_delete`

Remove a conversation and its transcript.

Delete a conversation. The messages go with it.

The conversation has to exist. Reporting success for an identifier this
workspace has never had would hide the case that actually happens — a caller
deleting against the wrong workspace — behind an answer that reads like it
worked.

- throw away a scratch thread

### `chats_get`

Read one conversation.

Read a conversation with its whole transcript, including what each attempt
to answer cost in tokens.

- read a conversation

### `chats_list`

List conversations.

List the conversations, most recently updated first.

The transcripts are left out: this answers what conversations exist, and
returning every message to answer it would be the most expensive call in the
system. Read one with `chats get`.

- the recent conversations
- only direct messages

### `chats_react`

Toggle a reaction on a message.

Leave a mark on one message, or take it back.

Sending the same reaction twice removes it — which is what clicking the same
emoji twice means, and what keeps one actor from stacking three identical
marks on one message.

Who is reacting comes from the identity of the call, never from the payload:
a caller that could name the actor could react as somebody else.

- agree with an answer

### `chats_send`

Write to a conversation.

Append a message and hand the turn to whoever should answer.

The message is persisted before anything is dispatched, so a runtime that dies
in between loses the answer and not the question. The result says which agent
was chosen and why, and whether a turn actually started — a message stored with
nobody to answer it is a real state, and one worth knowing about rather than
waiting through.

- ask whoever is listening
- ask a specific agent

### `chats_stop`

Stop the turn running on a conversation.

Ask the agent working on this conversation to stop.

The turn ends where it is: what it had already written is kept, and the
attempt is recorded as interrupted rather than as an error, because somebody
stopping an agent is not the agent failing.

Answering `stopped: false` is not a refusal. A person presses this
when they see an agent working, and by the time the call lands the turn may
have finished on its own.

- it is going the wrong way

### `chats_update`

Rename a conversation, or change who can read it.

Change a conversation's name or its visibility.

Only what you name changes. There is no way to send a whole conversation back
and have it written, because that would let a rename drop the transcript or
reopen a private thread to the workspace without anybody asking for it.

Visibility has a consequence worth stating: `workspace` makes the
transcript readable by every member, and there is no undo for what somebody has
already read.

- rename it
- open it to the workspace

