// Package comment is the discussion inside a task, and the channel an agent
// working autonomously reports progress through.
//
// The rule that makes it more than a text field is server-side authorship: the
// author comes from the ambient identity of the request, never from the
// payload, and an agent can only edit what it wrote. That is one of the few
// authorisation rules the original actually enforces in the domain, and it is
// what makes a task's discussion attributable.
package comment

import "time"

// Comment is one message in a task's discussion.
type Comment struct {
	// TaskID and ID come from the path: a comment lives at
	// .aos/tasks/{taskId}/comments/{id}.comment.md.
	TaskID string `yaml:"-" json:"taskId" collection:"path" jsonschema:"Identifier of the task this comment belongs to."`
	ID     string `yaml:"-" json:"id" collection:"path" jsonschema:"Identifier of this comment."`

	// Author is set from the ambient identity. There is no input field that
	// writes it, and adding one would be the bug.
	Author     string `yaml:"author" json:"author" jsonschema:"Who wrote it. Set by the server from the identity of the request."`
	AuthorType string `yaml:"authorType" json:"authorType" jsonschema:"agent or user."`

	// ParentID makes this a reply. Threads are one level deep: a reply to a
	// reply attaches to the same top-level comment.
	ParentID string `yaml:"parentId,omitempty" json:"parentId,omitempty" jsonschema:"The top-level comment this replies to."`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt" jsonschema:"When it was written."`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt" jsonschema:"When it was last edited."`

	Content string `yaml:"-" json:"content" collection:"content" jsonschema:"The comment body, in Markdown."`
}

// IsReply reports whether this comment hangs off another.
func (c Comment) IsReply() bool { return c.ParentID != "" }

// Thread is one top-level comment with its replies, which is how a discussion
// is read rather than as a flat list in write order.
type Thread struct {
	Comment Comment   `json:"comment" jsonschema:"The top-level comment."`
	Replies []Comment `json:"replies,omitempty" jsonschema:"Replies to it, oldest first."`
}
