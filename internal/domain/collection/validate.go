package collection

import (
	"fmt"
	"time"
)

// Exists reports whether a collection name is known. It is a parameter rather
// than a registry this package holds, because internal/domain does no lookups
// of its own — the caller in the service knows the registry.
type Exists func(name string) bool

// ValidateSchema checks that a declaration is coherent before it is stored: a
// ref has a target, the target exists, an enum has values, and no field is
// declared twice.
func ValidateSchema(c Collection, exists Exists) error {
	if len(c.Fields) == 0 {
		return errNoFields(c.ID)
	}
	seen := map[string]bool{}
	for _, f := range c.Fields {
		if f.Name == "" {
			return errFieldUnnamed(c.ID)
		}
		if seen[f.Name] {
			return errFieldDuplicated(c.ID, f.Name)
		}
		seen[f.Name] = true

		switch f.Type {
		case TypeString, TypeNumber, TypeBoolean, TypeDate, TypeList:
		case TypeEnum:
			if len(f.Enum) == 0 {
				return errEnumEmpty(c.ID, f.Name)
			}
		case TypeRef:
			if f.Ref == "" {
				return errRefMissing(c.ID, f.Name)
			}
			if exists != nil && !exists(f.Ref) {
				return errRefUnknown(c.ID, f.Name, f.Ref)
			}
		default:
			return errFieldTypeUnknown(c.ID, f.Name, string(f.Type))
		}
	}
	return nil
}

// Validate checks a record against the declared fields.
//
// existing is what is already stored, and it is a parameter for the same reason
// Exists is: the domain does not read the disk. Pass nil when there is nothing
// to compare against — uniqueness then has nothing to violate.
func Validate(c Collection, data map[string]any, existing []map[string]any) error {
	byName := make(map[string]Field, len(c.Fields))
	for _, f := range c.Fields {
		byName[f.Name] = f
	}

	// An undeclared field is refused rather than dropped. Dropping it would
	// mean an agent writes a record, reads it back, and finds part of what it
	// wrote missing with nothing having said so.
	for name := range data {
		if _, ok := byName[name]; !ok {
			return errFieldUndeclared(c.ID, name)
		}
	}

	for _, f := range c.Fields {
		raw, present := data[f.Name]
		if !present || raw == nil {
			if f.Required {
				return errFieldRequired(c.ID, f.Name)
			}
			continue
		}
		if err := checkType(c.ID, f, raw); err != nil {
			return err
		}
		if f.Unique {
			for _, other := range existing {
				if fmt.Sprint(other[f.Name]) == fmt.Sprint(raw) {
					return errFieldNotUnique(c.ID, f.Name, raw)
				}
			}
		}
	}
	return nil
}

func checkType(id string, f Field, raw any) error {
	switch f.Type {
	case TypeString:
		if _, ok := raw.(string); !ok {
			return errFieldWrongType(id, f.Name, "string", raw)
		}
	case TypeNumber:
		switch raw.(type) {
		case float64, float32, int, int64:
		default:
			return errFieldWrongType(id, f.Name, "number", raw)
		}
	case TypeBoolean:
		if _, ok := raw.(bool); !ok {
			return errFieldWrongType(id, f.Name, "boolean", raw)
		}
	case TypeDate:
		s, ok := raw.(string)
		if !ok {
			return errFieldWrongType(id, f.Name, "date", raw)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return errFieldWrongType(id, f.Name, "date in RFC 3339", raw)
		}
	case TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return errFieldWrongType(id, f.Name, "string", raw)
		}
		for _, allowed := range f.Enum {
			if s == allowed {
				return nil
			}
		}
		return errFieldNotInEnum(id, f.Name, s, f.Enum)
	case TypeRef:
		if _, ok := raw.(string); !ok {
			return errFieldWrongType(id, f.Name, "the id of a record, as a string", raw)
		}
	case TypeList:
		if _, ok := raw.([]any); !ok {
			return errFieldWrongType(id, f.Name, "list", raw)
		}
	}
	return nil
}
