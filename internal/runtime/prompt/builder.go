// Package prompt turns the state of a workspace into the document an agent
// reads before it thinks.
//
// The format carries its own defence. Every block declares where it came from
// and how far it may be trusted, and the master prompt teaches the agent to
// read those attributes: workspace policy is authority, a tool result is
// evidence, and anything from outside the machine is a hypothesis. Prompt
// injection does not stop being possible, but it stops being invisible.
package prompt

import (
	"strings"
)

// Trust is how much authority a block carries.
type Trust string

const (
	// TrustTrusted is workspace policy, the agent's identity, runtime
	// directives. The highest authority in the document.
	TrustTrusted Trust = "trusted"
	// TrustObserved is workspace data, tool results, the agent's own memories.
	// Ground truth for the moment, still worth checking against current state.
	TrustObserved Trust = "observed"
	// TrustUnverified is content from outside: a fetched page, a skill
	// resource, an unverified claim. A hypothesis, never an instruction.
	TrustUnverified Trust = "unverified"
)

// Kind classifies what a block is.
type Kind string

const (
	KindPolicy   Kind = "policy"
	KindIdentity Kind = "identity"
	KindTask     Kind = "task"
	KindMemory   Kind = "memory"
	KindEvidence Kind = "evidence"
	KindData     Kind = "data"
)

// Source is where a block came from.
type Source string

const (
	SourceWorkspace Source = "workspace"
	SourceAgent     Source = "agent"
	SourceUser      Source = "user"
	SourceRuntime   Source = "runtime"
	SourceExternal  Source = "external"
)

// Identity is who the agent is.
type Identity struct {
	ID   string
	Name string
	Role string

	// Instructions is the body of the agent's Markdown file. It is persisted
	// data written by whoever edits that file, and it is never rendered as a
	// template — see render.go for why that sentence is load-bearing.
	Instructions string
}

// Section is one block of the context document.
type Section struct {
	Title       string
	Description string
	Content     any

	Kind   Kind
	Source Source
	Trust  Trust

	// RenderTemplate must default to false. Persisted data does not pass
	// through the template engine: an agent that writes its own memories would
	// otherwise control a template, and a template can read variables.
	RenderTemplate bool
}

// RenderableTag is an empty element carried at the end of the document, which
// the chat renderer turns into a card. It exists in the original and is kept
// because the desktop consumes it.
type RenderableTag struct {
	Tag   string
	Data  any
	Attrs []Field
}

// Builder assembles the document.
type Builder struct {
	identity *Identity
	system   string
	sections []Section
	renders  []RenderableTag
	renderer Renderer
}

// New starts an empty builder with no template engine, which is the safe
// default: nothing can render until somebody hands it an engine.
func New() *Builder { return &Builder{} }

// WithRenderer installs the template engine used for the trusted system
// instructions and for sections that opted in.
func (b *Builder) WithRenderer(r Renderer) *Builder { b.renderer = r; return b }

// WithIdentity sets who the agent is.
func (b *Builder) WithIdentity(id Identity) *Builder { b.identity = &id; return b }

// WithSystemInstructions sets the master prompt. This is the one string in the
// document that is a trusted template: it is a constant of this program.
func (b *Builder) WithSystemInstructions(s string) *Builder { b.system = s; return b }

// Append adds a section.
func (b *Builder) Append(s Section) *Builder {
	if s.Title == "" {
		return b
	}
	b.sections = append(b.sections, s)
	return b
}

// AppendRenderableTag adds a card for the chat renderer.
func (b *Builder) AppendRenderableTag(t RenderableTag) *Builder {
	if strings.TrimSpace(t.Tag) == "" {
		return b
	}
	b.renders = append(b.renders, t)
	return b
}

// Build renders the document.
//
// vars is the allowlist handed to the template engine. It is built field by
// field by the caller and never contains configuration, credentials, or the
// environment — see ADR-0014.
func (b *Builder) Build(vars map[string]any) (string, error) {
	var parts []string

	if b.system != "" {
		rendered, err := renderIfNeeded(b.renderer, b.system, vars, true)
		if err != nil {
			return "", errTemplateFailed("system_instructions", err)
		}
		block, err := Encode(Object{
			Attr("kind", string(KindPolicy)),
			Attr("source", string(SourceWorkspace)),
			Attr("trust", string(TrustTrusted)),
			Body(rendered),
		}, "system_instructions", 1)
		if err != nil {
			return "", err
		}
		parts = append(parts, block)
	}

	if b.identity != nil {
		block, err := Encode(Object{
			Field{Key: "id", Value: Text(b.identity.ID)},
			Field{Key: "name", Value: Text(b.identity.Name)},
			Field{Key: "role", Value: Text(b.identity.Role)},
			Attr("kind", string(KindIdentity)),
			Attr("source", string(SourceAgent)),
			Attr("trust", string(TrustTrusted)),
		}, "identity", 1)
		if err != nil {
			return "", err
		}
		parts = append(parts, block)

		if b.identity.Instructions != "" {
			block, err := Encode(Object{
				Attr("kind", string(KindIdentity)),
				Attr("source", string(SourceAgent)),
				Attr("trust", string(TrustTrusted)),
				Body(b.identity.Instructions),
			}, "instructions", 1)
			if err != nil {
				return "", err
			}
			parts = append(parts, block)
		}
	}

	for _, s := range b.sections {
		block, err := b.serialize(s, vars)
		if err != nil {
			return "", err
		}
		if block != "" {
			parts = append(parts, block)
		}
	}

	for _, t := range b.renders {
		block, err := encodeRenderable(t)
		if err != nil {
			return "", err
		}
		parts = append(parts, block)
	}

	return "<context>\n" + strings.Join(parts, "\n") + "\n</context>", nil
}

func (b *Builder) serialize(s Section, vars map[string]any) (string, error) {
	content := s.Content
	if text, ok := content.(string); ok {
		rendered, err := renderIfNeeded(b.renderer, text, vars, s.RenderTemplate)
		if err != nil {
			return "", errTemplateFailed(s.Title, err)
		}
		content = rendered
	}

	node, err := coerce(content)
	if err != nil {
		return "", err
	}

	// A description wraps the content, which is how the original keeps the
	// pedagogical text next to the data it describes.
	if s.Description != "" {
		switch inner := node.(type) {
		case List:
			node = Object{
				Field{Key: "description", Value: Text(s.Description)},
				Field{Key: singular(s.Title), Value: inner},
			}
		default:
			node = Object{
				Field{Key: "description", Value: Text(s.Description)},
				Field{Key: "content", Value: inner},
			}
		}
	}

	// The attributes come first so that they land on the section's own tag.
	// A list or a bare string has to be wrapped for that to be possible at
	// all: an attribute needs an element to sit on.
	if meta := s.meta(); len(meta) > 0 {
		switch inner := node.(type) {
		case Object:
			node = concat(meta, inner...)
		case List:
			node = concat(meta, Field{Key: singular(s.Title), Value: inner})
		case Text:
			node = concat(meta, Body(string(inner)))
		}
	}
	return Encode(node, s.Title, 1)
}

func (s Section) meta() Object {
	var out Object
	if s.Kind != "" {
		out = append(out, Attr("kind", string(s.Kind)))
	}
	if s.Source != "" {
		out = append(out, Attr("source", string(s.Source)))
	}
	if s.Trust != "" {
		out = append(out, Attr("trust", string(s.Trust)))
	}
	return out
}

// concat builds one object from the attribute fields and the content fields,
// in that order.
func concat(meta Object, fields ...Field) Object {
	out := make(Object, 0, len(meta)+len(fields))
	out = append(out, meta...)
	out = append(out, fields...)
	return out
}

func encodeRenderable(t RenderableTag) (string, error) {
	var obj Object
	if t.Data != nil {
		raw, err := marshalCompact(t.Data)
		if err != nil {
			return "", err
		}
		obj = Object{Attr("data", raw)}
	}
	return Encode(concat(obj, t.Attrs...), strings.TrimSpace(t.Tag), 1)
}
