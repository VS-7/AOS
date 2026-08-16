package comment

import "github.com/OWNER/aos/internal/core/command"

// ListInput reads one task's discussion.
type ListInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`

	command.Reasoning
}

// ListOutput carries the discussion both ways it is read: flat, in write order,
// and grouped into threads.
type ListOutput struct {
	Comments []Comment `json:"comments" jsonschema:"Every comment, oldest first."`
	Threads  []Thread  `json:"threads" jsonschema:"The same comments grouped: each top-level comment with its replies."`
	Total    int       `json:"total" jsonschema:"How many comments the task has."`
}

// GetInput names one comment.
type GetInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the comment." validate:"required,notblank"`

	command.Reasoning
}

// CreateInput writes a comment.
//
// There is deliberately no author field. Authorship is server-side: it comes
// from the identity of the request, and a payload that could set it would make
// the discussion history worthless as a record of who said what.
type CreateInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	Body string `json:"body" cli:"arg" jsonschema:"The comment, in Markdown." validate:"required,notblank"`

	Parent string `json:"parent,omitempty" cli:"flag" jsonschema:"Comment this replies to. A reply to a reply joins the same thread."`

	command.Reasoning
}

// UpdateInput rewrites a comment's body.
type UpdateInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the comment." validate:"required,notblank"`
	Body string `json:"body" cli:"arg" jsonschema:"The new body, in Markdown." validate:"required,notblank"`

	command.Reasoning
}

// DeleteInput removes a comment.
type DeleteInput struct {
	Task string `json:"task" cli:"arg" jsonschema:"Identifier of the parent task." validate:"required,notblank"`
	ID   string `json:"id" cli:"arg" jsonschema:"Identifier of the comment." validate:"required,notblank"`

	command.Reasoning
}

// DeleteOutput names what went and what was left standing.
type DeleteOutput struct {
	ID   string `json:"id" jsonschema:"The comment that was removed."`
	Task string `json:"task" jsonschema:"The task it belonged to."`

	// Promoted lists the replies that were moved to the top level rather than
	// removed with their parent. Cascading would let one participant erase
	// another's words by deleting the message they were answering.
	Promoted []string `json:"promoted,omitempty" jsonschema:"Replies that were promoted to the top level instead of being removed."`
}
