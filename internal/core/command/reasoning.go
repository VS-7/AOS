package command

import (
	"reflect"
	"strings"
)

// ReasoningField is the JSON name of the mandatory reasoning field.
//
// The name, the description and the rejection message are copied verbatim from
// the original's AgentSchema.object. They are not decoration: they are what the
// model reads, and a model that has seen this exact wording elsewhere fills the
// field correctly on the first attempt.
const ReasoningField = "_reasoning"

// ReasoningDescription is what the model reads in the tool schema.
const ReasoningDescription = "MANDATORY. NEVER FORGET. Explain why this specific tool is being called now, " +
	"what outcome you expect, and the immediate next step if that helps clarify the call. Do not leave this empty."

// ReasoningRejection is the validation message of an empty reasoning.
const ReasoningRejection = "_reasoning is MANDATORY — explain why this specific tool is being called now, " +
	"what outcome you expect, and the immediate next step. An empty string is a rejected call."

// Reasoning is embedded in every command input.
//
// The specification writes the field out in each input struct. Embedding it
// means the wording, the validation rule and the schema description exist once
// — and a struct that forgets to embed it fails at registration rather than at
// the first tool call.
type Reasoning struct {
	Reasoning string `json:"_reasoning" jsonschema:"MANDATORY. NEVER FORGET. Explain why this specific tool is being called now, what outcome you expect, and the immediate next step if that helps clarify the call. Do not leave this empty." validate:"required,min=1"`
}

// reasoningFieldPath returns the validator path of the reasoning field inside
// an input type, or "" when the type does not have one.
func reasoningFieldPath(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if jsonNameOf(f) == ReasoningField {
			return f.Name
		}
		if f.Anonymous {
			if inner := reasoningFieldPath(f.Type); inner != "" {
				return f.Name + "." + inner
			}
		}
	}
	return ""
}

// HasReasoning reports whether an input type carries the mandatory field.
func HasReasoning(t reflect.Type) bool { return reasoningFieldPath(t) != "" }

func jsonNameOf(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	return name
}

// isSurfacePrivate reports whether a field belongs to the tool surface only and
// must not become a CLI flag. `_reasoning` is the only one today: asking a human
// at a terminal to justify their own command would be absurd, and the original
// does not do it either — its CLI schema is args plus options, and the tool
// schema is that plus `_reasoning`.
func isSurfacePrivate(name string) bool { return strings.HasPrefix(name, "_") }
