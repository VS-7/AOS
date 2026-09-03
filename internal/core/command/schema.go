package command

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// completeSchema repairs and verifies an inferred schema.
//
// The inference library silently drops the fields of an embedded struct: a type
// that embeds command.Reasoning comes back with no `_reasoning` property at
// all. Silently, which is the dangerous part — the tool would publish a schema
// that does not mention the one field every call must carry, and no model would
// ever send it.
//
// So two things happen here. The reasoning property is injected when it is
// missing, which is what "injected automatically by reflection over In at
// registration" means in practice. And every other JSON-visible field is
// checked against the schema, so the next field the library drops fails the
// build instead of disappearing from the contract.
// CompleteSchema is completeSchema, for the one caller outside this package
// that has to describe an input the same way: the agent's native tools
// (internal/runtime/toolexec/tools).
//
// They infer their schema the same way and were not repairing it, so the six
// tools a model reaches for most — Read, Write, Edit, Glob, Grep, Bash —
// published no `_reasoning` at all. The master prompt requires one on every
// call; the schema said otherwise and, being `additionalProperties: false`,
// made sending one a violation. A tool that is exempt from the contract every
// other tool is held to is a hole in the contract, not an exemption.
func CompleteSchema(t reflect.Type, s *jsonschema.Schema) error { return completeSchema(t, s) }

func completeSchema(t reflect.Type, s *jsonschema.Schema) error {
	if s.Properties == nil {
		s.Properties = map[string]*jsonschema.Schema{}
	}

	if _, ok := s.Properties[ReasoningField]; !ok && HasReasoning(t) {
		s.Properties[ReasoningField] = &jsonschema.Schema{
			Type:        "string",
			MinLength:   jsonschema.Ptr(1),
			Description: ReasoningDescription,
		}
	}
	if _, ok := s.Properties[ReasoningField]; ok && !contains(s.Required, ReasoningField) {
		s.Required = append(s.Required, ReasoningField)
	}

	var missing []string
	for _, field := range jsonFields(t) {
		if _, ok := s.Properties[field]; !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("the inferred schema is missing %s — "+
			"the inference library dropped them, and a published schema that omits a field is a lie",
			strings.Join(missing, ", "))
	}

	publishEnums(t, s)
	openRawMessages(t, s)
	return nil
}

// rawMessageType is json.RawMessage, which is a []byte and infers as one.
var rawMessageType = reflect.TypeOf(json.RawMessage(nil))

// openRawMessages replaces the schema of every json.RawMessage field with one
// that accepts anything.
//
// A RawMessage is "whatever JSON the caller sends, kept verbatim" — the
// arguments of an external tool, a view's component tree, the payload a person
// corrected before approving. The inference library sees the underlying
// []byte and publishes an array of integers between 0 and 255, which is what
// the model was told to send: `toolsets_call.input` was described as a byte
// array, so no schema-following model could pass arguments to an external tool
// at all, and a strict provider would refuse an object outright.
//
// The daemon accepted objects anyway, because RawMessage unmarshals anything —
// which is exactly why nothing caught it: the contract was wrong only where a
// model could read it.
func openRawMessages(t reflect.Type, s *jsonschema.Schema) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous && f.Tag.Get("json") == "" {
			openRawMessages(f.Type, s)
			continue
		}
		if f.Type != rawMessageType {
			continue
		}
		name := jsonNameOf(f)
		if name == "" {
			continue
		}
		// Keep the description — it is the only guidance the model has about
		// what shape belongs there — and drop the type constraint.
		description := ""
		if existing, ok := s.Properties[name]; ok && existing != nil {
			description = existing.Description
		}
		s.Properties[name] = &jsonschema.Schema{Description: description}
	}
}

// publishEnums copies every closed set the validator already enforces into the
// schema that publishes it.
//
// `validate:"oneof=a b c"` is the single declaration of an accepted set: the
// validator refuses anything else and the refusal names the values. Until this
// existed, that refusal was the *only* place they appeared — not in the schema,
// not in --help — so the way to learn which categories a memory accepts was to
// store one with the wrong category first (defect #8).
func publishEnums(t reflect.Type, s *jsonschema.Schema) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous && f.Tag.Get("json") == "" {
			publishEnums(f.Type, s)
			continue
		}
		name := jsonNameOf(f)
		if name == "" {
			continue
		}
		prop, published := s.Properties[name]
		if !published || prop == nil || len(prop.Enum) > 0 {
			continue
		}
		if values := enumOf(f.Type); len(values) > 0 {
			prop.Enum = values
			continue
		}
		if values := oneOfValues(f.Tag.Get("validate")); len(values) > 0 {
			prop.Enum = values
		}
	}
}

// Enumerator is implemented by a domain type whose values are a closed set.
//
// It exists so that the set is declared once, by the type that owns it, and
// read by everything that needs it: the type's own Valid(), the schema every
// surface publishes, and the refusal a wrong value earns. The alternative — a
// list in the validate tag next to a list in the domain — is two declarations
// of one truth, and the schema is the copy nobody would notice going stale.
//
// Enforcement deliberately stays in the domain: a category the memory service
// refuses answers with MEMORY_INVALID_CATEGORY and its own call to action,
// which is more use to a caller than a generic validation failure.
type Enumerator interface {
	// EnumValues lists every accepted value, in the order the domain declares
	// them.
	EnumValues() []string
}

// enumOf reads the closed set a field's type declares, if it declares one.
func enumOf(t reflect.Type) []any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	e, ok := reflect.Zero(t).Interface().(Enumerator)
	if !ok {
		return nil
	}
	values := e.EnumValues()
	out := make([]any, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// oneOfValues reads the accepted set out of a validate tag, which spells it
// `oneof=a b c` among comma-separated rules.
func oneOfValues(tag string) []any {
	for _, rule := range strings.Split(tag, ",") {
		spec, found := strings.CutPrefix(strings.TrimSpace(rule), "oneof=")
		if !found {
			continue
		}
		out := []any{}
		for _, v := range strings.Fields(spec) {
			out = append(out, v)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// jsonFields lists the JSON names a value of this type serialises to, following
// encoding/json's rules for embedded structs.
func jsonFields(t reflect.Type) []string {
	var out []string
	collectJSONFields(t, &out)
	return out
}

func collectJSONFields(t reflect.Type, out *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous && f.Tag.Get("json") == "" {
			collectJSONFields(f.Type, out)
			continue
		}
		if name := jsonNameOf(f); name != "" {
			*out = append(*out, name)
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
