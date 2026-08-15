package command

import (
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
