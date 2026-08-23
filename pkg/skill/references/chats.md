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

### `chats_create`

Open a conversation.

Open a conversation.

Whoever opens it is in it. A private conversation its own creator could not read
is not something anyone means to make.

- a workspace channel
- a private thread with one agent

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

