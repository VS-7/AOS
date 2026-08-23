# Comments

The discussion inside a task.

Progress, questions and feedback on a task.

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
- Not for what you learned in general — that is a memory

## Commands

### `comments_create`

Write a comment on a task.

Post to a task's discussion.

You are the author because of who is making the call; there is no field for it.
Pass a parent to reply — and a reply to a reply joins the same thread rather
than nesting deeper.

- report progress while executing
- answer in a thread

### `comments_delete`

Remove a comment you wrote.

Remove your own comment.

Its replies survive, promoted to the top level: deleting the message somebody
was answering must not delete their answer.

- remove a comment posted by mistake

### `comments_get`

Read one comment.

Read one comment in full, with its author and the thread it belongs to.

- read a comment by identifier

### `comments_list`

Read a task's discussion.

Every comment on a task, oldest first, and the same comments grouped
into threads.

Both shapes are returned because both are needed: the flat list is the order
things were said, and the threads are how the discussion reads.

- catch up on a task

### `comments_update`

Rewrite a comment you wrote.

Change the body of your own comment.

Somebody else's comment is not yours to edit, and the attempt is refused. Reply
to it instead.

- correct your own comment

